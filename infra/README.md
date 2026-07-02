# Deploying stevio-home

Target: a single **Flatcar Container Linux** VM on Hetzner Cloud (x86_64),
running the app via docker-compose from pre-built GHCR images. Caddy terminates
TLS; Postgres runs on-box with scheduled backups.

```
Internet ──▶ Caddy (:443, Let's Encrypt) ──▶ frontend nginx (:80) ──▶ backend (:8080)
                                                                          └─▶ postgres (:5432)
```

The steps below go in order: **prerequisites → Ignition → install Flatcar →
first boot → build images + token → copy files → start → verify.** Do them once
for the initial go-live; day-2 tasks (update, backups, OS updates) are at the end.

---

## 1. Prerequisites (local workstation)

```bash
brew install hcloud butane        # macOS; hcloud CLI + Butane compiler
hcloud context create stevio      # paste a Read&Write API token (Hetzner Console → Security → API Tokens)
```

Ensure you have an SSH key and register it in the Hetzner project (used for the
initial server + the rescue system):

```bash
ls ~/.ssh/*.pub || ssh-keygen -t ed25519 -C "stevio-flatcar"
hcloud ssh-key create --name stevio --public-key-from-file ~/.ssh/id_ed25519.pub
```

## 2. Build the Ignition config

Secrets and your SSH key go into a **local, gitignored** file — never into the
tracked `butane.yaml`:

```bash
cp infra/flatcar/butane.yaml infra/flatcar/butane.local.yaml
```

Edit `infra/flatcar/butane.local.yaml`:
- Replace `REPLACE_WITH_YOUR_SSH_PUBLIC_KEY` with your `~/.ssh/id_ed25519.pub`
  (required — Flatcar has no password login; without it you can't SSH in).
- Replace every `CHANGE_ME`. Generate the three secrets from a Go checkout:
  ```bash
  just gensigningsecret   # -> SIGNING_KEY_SECRET
  just gensecret          # -> SESSION_SECRET
  just gensalt            # -> EMAIL_HASH_SALT
  ```
  Also set `POSTGRES_PASSWORD`, `DOMAIN`, `BASE_URL` (https), `ACME_EMAIL`,
  `ADMIN_EMAILS`, optional `SMTP_*`, and `IMAGE_TAG` (`stable` or a pinned
  `sha-<short>`).

Compile Ignition **from the local file**:

```bash
butane --strict infra/flatcar/butane.local.yaml > infra/flatcar/ignition.json
```

`butane.local.yaml` and `ignition.json` hold real secrets and are gitignored —
never commit them. Because the secrets are embedded in Ignition, the server comes
up with `/etc/stevio/stevio.env` already filled (mode 0600).

## 3. Install Flatcar on the Hetzner VM (rescue method)

Hetzner Cloud has **no Flatcar image**, and Flatcar on Hetzner does not read
Ignition from user-data — so we install via the rescue system and embed Ignition
into the image at install time.

**Create a new server** (any image; it gets overwritten) — or reuse an existing one:

```bash
hcloud server create --name stevio --type cx22 --location nbg1 \
  --image ubuntu-24.04 --ssh-key stevio
```

**Enable rescue and reboot into it:**

```bash
hcloud server enable-rescue stevio --ssh-key stevio
hcloud server reboot stevio
```

> Web alternative: Server → **Rescue** tab → *Enable rescue* (`linux64`, select your
> SSH key) → **Power → Reboot**.

**Copy Ignition in, then install** (⚠️ overwrites `/dev/sda` completely):

```bash
scp infra/flatcar/ignition.json root@<SERVER_IP>:/root/ignition.json
ssh root@<SERVER_IP>

# inside the rescue system:
lsblk                                   # confirm the target disk is /dev/sda
curl -fsSLO https://raw.githubusercontent.com/flatcar/init/flatcar-master/bin/flatcar-install
chmod +x flatcar-install
./flatcar-install -d /dev/sda -o hetzner -i /root/ignition.json
#   if -o hetzner errors: rerun without it — Ignition still applies (embedded via -i)
reboot
```

## 4. First boot verification

Wait ~30–60s, then log in as **`core`** (not `root`):

```bash
ssh core@<SERVER_IP>
docker --version
cat /etc/stevio/stevio.env           # your secrets/config should be present (0600)
```

The Butane config installs the Docker Compose plugin automatically on first boot
(`docker-compose-install.service`) — Flatcar ships Docker but not Compose, since
`/usr` is read-only. If you provisioned an older config without that unit, install
it manually:

```bash
COMPOSE_VERSION=v2.29.7               # check github.com/docker/compose/releases
sudo mkdir -p /root/.docker/cli-plugins ~/.docker/cli-plugins
sudo curl -fsSL -o /root/.docker/cli-plugins/docker-compose \
  https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-x86_64
sudo chmod +x /root/.docker/cli-plugins/docker-compose
sudo cp /root/.docker/cli-plugins/docker-compose ~/.docker/cli-plugins/docker-compose
sudo chown core:core ~/.docker/cli-plugins/docker-compose
sudo docker compose version
```

`stevio.service` will still fail at this point — expected, because the deploy
files aren't on the host yet (step 6).

## 5. Build images in GHCR + create a pull token

Images must exist in the registry before the host can pull them.

1. **Push the repo to trigger CI.** The workflow builds on push to `main`:
   ```bash
   git status          # confirm butane.local.yaml + ignition.json do NOT appear (gitignored)
   git add -A
   git commit -m "add deployment infra"
   git push origin main
   ```
   Watch **Actions → “Build & push images”**. When green, two **private** packages
   appear under your profile → Packages: `stevio-home-backend`, `stevio-home-frontend`.

2. **Registry access.** Either:
   - **Public packages (simplest):** Package → *Settings → Change visibility →
     Public*. The host pulls with no login; skip the `docker login` below. Images
     contain no secrets (those come via env at runtime). *Or:*
   - **Private + token:** GitHub → *Settings → Developer settings → Personal access
     tokens → Tokens (classic)* → scope **`read:packages` only** → generate. Then on
     the server (must be `sudo` — the service runs as root):
     ```bash
     ssh core@<SERVER_IP>
     sudo docker login ghcr.io -u <your-github-username>   # password = the token
     ```
     Stored in `/root/.docker/config.json`, persists across reboots.

## 6. Copy deploy files to the host

`/opt/stevio` is owned by root, so stage in `/tmp` then move:

```bash
# from the repo root on your workstation:
scp docker-compose.deploy.yml Caddyfile core@<SERVER_IP>:/tmp/
ssh core@<SERVER_IP> 'sudo cp /tmp/docker-compose.deploy.yml /tmp/Caddyfile /opt/stevio/'
```

## 7. DNS

Point your domain at the server so Caddy can obtain a certificate (Let's Encrypt
HTTP challenge needs the name to resolve and ports 80/443 open):

- `A` record `DOMAIN` → server IPv4  (and optional `AAAA` → IPv6)
- verify: `dig +short <DOMAIN>`

## 8. Start & verify

```bash
ssh core@<SERVER_IP>
sudo systemctl restart stevio
sudo docker compose -f /opt/stevio/docker-compose.deploy.yml --env-file /etc/stevio/stevio.env ps
sudo docker compose -f /opt/stevio/docker-compose.deploy.yml --env-file /etc/stevio/stevio.env logs -f
```

Expected log order: **postgres healthy → backend runs migrations → `/healthz`
green → Caddy issues the certificate for `DOMAIN`.** Then browse to
`https://<DOMAIN>`, log in with an `ADMIN_EMAILS` address (magic link), and
configure the payment provider at `/admin/payment`.

---

## Updating (deliberate, healthcheck-gated)

Bump the pinned tag and restart — Postgres migrations run automatically at
backend startup:

```bash
sudo sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=sha-<newshort>/' /etc/stevio/stevio.env
sudo systemctl restart stevio
```

**Rollback:** set `IMAGE_TAG` back to the previous `sha-…` and restart. (Schema
migrations are forward-only; a rollback that spans a migration needs care — for a
store this size, prefer rolling forward with a fix.)

## Off-site backups

The `backup` service writes rotated `pg_dump`s to `/data/stevio/backups`. To ship
them off-box, put an rclone remote named `offsite` in
`/data/stevio/rclone/rclone.conf` (Hetzner Storage Box via `sftp`/`webdav`, or any
S3-compatible target), then:

```bash
sudo docker compose -f /opt/stevio/docker-compose.deploy.yml --env-file /etc/stevio/stevio.env --profile offsite up -d
```

### Restore drill (do this once before you rely on it)

```bash
ls /data/stevio/backups/daily
gunzip -c /data/stevio/backups/daily/<file>.sql.gz \
  | sudo docker compose -f /opt/stevio/docker-compose.deploy.yml --env-file /etc/stevio/stevio.env exec -T postgres \
      psql -U stevio -d stevio
```

## OS auto-updates

Flatcar auto-updates the OS and reboots within the **03:00–05:00** window
(configured via locksmithd in the Butane config). `restart: unless-stopped` on
every service plus Postgres durability means a reboot is non-disruptive beyond a
few seconds of downtime. To change the window, edit the locksmithd drop-in.
