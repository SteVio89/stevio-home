package store

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/markdown"
)

// ListProjects handles GET /api/projects. Returns the public project listing
// with attached commerce data (price, bundle id, purchase mode) and translated
// title/tagline. Description is intentionally NOT included in the listing —
// fetched per-project on the detail page.
func (h *StoreHandler) ListProjects(c *app.Ctx) error {
	ctx := c.R.Context()
	defaultLoc := c.Locales().Default(ctx)
	loc := c.Lang()

	projects, err := queries.ListProjectsPublic(ctx, c.DB().DB)
	if err != nil {
		h.log.Printf("store: list projects: %v", err)
		return apierr.ErrInternal()
	}

	// Apply default locale translations as base, then overlay requested locale.
	if defTrans, err := i18n.GetEntityTranslationsForLocale(ctx, c.DB().DB, common.EntityTypeProject, defaultLoc); err == nil {
		for i := range projects {
			applyProjectOverlay(&projects[i], defTrans[projects[i].ID])
		}
	}
	if loc != defaultLoc {
		if trans, err := i18n.GetEntityTranslationsForLocale(ctx, c.DB().DB, common.EntityTypeProject, loc); err == nil {
			for i := range projects {
				applyProjectOverlay(&projects[i], trans[projects[i].ID])
			}
		}
	}

	// Apply auto-discounts to commerce projects (final price → DiscountedPriceCents).
	if activeDiscounts, err := queries.GetAllActiveAutoDiscounts(ctx, c.DB().DB); err == nil && len(activeDiscounts) > 0 {
		for i := range projects {
			a := projects[i].Commerce
			if a == nil || a.PriceCents <= 0 {
				continue
			}
			best := bestAutoDiscount(activeDiscounts, a.ID)
			if best != nil {
				final := queries.ApplyDiscount(best.DiscountType, best.DiscountValue, a.PriceCents)
				a.DiscountedPriceCents = &final
			}
		}
	}

	return c.JSON(http.StatusOK, projects)
}

// GetProjectDetail handles GET /api/projects/{slug}. Renders the full detail
// page payload: translated title/tagline/description, gallery images (with
// alt-text translations), and — when commerce is attached — system
// requirements plus the latest 5 versions.
//
// Returns 404 when the project is missing or has has_detail_page = false
// (external-only projects link out, no detail page).
func (h *StoreHandler) GetProjectDetail(c *app.Ctx) error {
	ctx := c.R.Context()
	defaultLoc := c.Locales().Default(ctx)
	loc := c.Lang()
	slug := c.Param("slug")

	project, err := queries.GetProjectBySlug(ctx, c.DB().DB, slug)
	if err != nil {
		h.log.Printf("store: get project detail %q: %v", slug, err)
		return apierr.ErrInternal()
	}
	if project == nil {
		return apierr.ErrNotFound()
	}
	if !project.HasDetailPage {
		return apierr.ErrNotFound()
	}

	// Project text translations.
	if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeProject, project.ID, defaultLoc); err == nil {
		applyProjectOverlay(project, fields)
	}
	if loc != defaultLoc {
		if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeProject, project.ID, loc); err == nil {
			applyProjectOverlay(project, fields)
		}
	}
	project.Description = markdown.ToHTML(project.Description)

	// Gallery images + alt-text translations.
	images, err := queries.ListProjectImages(ctx, c.DB().DB, project.ID)
	if err != nil {
		h.log.Printf("store: list project images %q: %v", project.ID, err)
		return apierr.ErrInternal()
	}
	for idx := range images {
		imgID := images[idx].ID
		if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeProjectImage, imgID, defaultLoc); err == nil {
			i18n.NewOverlay(fields).Apply("alt_text", &images[idx].AltText)
		}
		if loc != defaultLoc {
			if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeProjectImage, imgID, loc); err == nil {
				i18n.NewOverlay(fields).Apply("alt_text", &images[idx].AltText)
			}
		}
		// file_path is admin-only, strip it from the public response.
		images[idx].FilePath = ""
	}
	project.Images = images

	// Commerce attachment: load app, system_requirements translation, and versions.
	commerce, err := queries.GetAppByProjectID(ctx, c.DB().DB, project.ID)
	if err != nil {
		h.log.Printf("store: get app for project %q: %v", project.ID, err)
		return apierr.ErrInternal()
	}
	if commerce != nil {
		if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeApp, commerce.ID, defaultLoc); err == nil {
			i18n.NewOverlay(fields).Apply("system_requirements", &commerce.SystemRequirements)
		}
		if loc != defaultLoc {
			if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeApp, commerce.ID, loc); err == nil {
				i18n.NewOverlay(fields).Apply("system_requirements", &commerce.SystemRequirements)
			}
		}
		commerce.SystemRequirements = markdown.ToHTML(commerce.SystemRequirements)

		// Auto-discount overlay (cheapest applicable wins).
		if activeDiscounts, err := queries.GetAllActiveAutoDiscounts(ctx, c.DB().DB); err == nil && len(activeDiscounts) > 0 && commerce.PriceCents > 0 {
			if best := bestAutoDiscount(activeDiscounts, commerce.ID); best != nil {
				final := queries.ApplyDiscount(best.DiscountType, best.DiscountValue, commerce.PriceCents)
				commerce.DiscountedPriceCents = &final
			}
		}

		project.Commerce = commerce

		versions, err := queries.ListVersionsByAppID(ctx, c.DB().DB, commerce.ID)
		if err != nil {
			h.log.Printf("store: list versions for project %q: %v", project.ID, err)
			return apierr.ErrInternal()
		}
		if len(versions) > 5 {
			versions = versions[:5]
		}
		for i := range versions {
			vid := versions[i].ID
			if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeVersion, vid, defaultLoc); err == nil {
				i18n.NewOverlay(fields).Apply("release_notes", &versions[i].ReleaseNotes)
			}
			if loc != defaultLoc {
				if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeVersion, vid, loc); err == nil {
					i18n.NewOverlay(fields).Apply("release_notes", &versions[i].ReleaseNotes)
				}
			}
			versions[i].ReleaseNotes = markdown.ToHTML(versions[i].ReleaseNotes)
			versions[i].FilePath = ""
		}
		project.Versions = versions
	}

	return c.JSON(http.StatusOK, project)
}

// GetProjectVersions handles GET /api/projects/{slug}/versions. Returns the
// (most recent 5) versions for a commerce project. 404 when the project is
// missing, soft-deleted, or has no commerce attached.
func (h *StoreHandler) GetProjectVersions(c *app.Ctx) error {
	ctx := c.R.Context()
	defaultLoc := c.Locales().Default(ctx)
	loc := c.Lang()
	slug := c.Param("slug")

	project, err := queries.GetProjectBySlug(ctx, c.DB().DB, slug)
	if err != nil {
		h.log.Printf("store: get project for versions %q: %v", slug, err)
		return apierr.ErrInternal()
	}
	if project == nil {
		return apierr.ErrNotFound()
	}
	commerce, err := queries.GetAppByProjectID(ctx, c.DB().DB, project.ID)
	if err != nil {
		h.log.Printf("store: get app for project versions %q: %v", project.ID, err)
		return apierr.ErrInternal()
	}
	if commerce == nil {
		return apierr.ErrNotFound()
	}

	versions, err := queries.ListVersionsByAppID(ctx, c.DB().DB, commerce.ID)
	if err != nil {
		h.log.Printf("store: list versions %q: %v", commerce.ID, err)
		return apierr.ErrInternal()
	}
	if len(versions) > 5 {
		versions = versions[:5]
	}
	for i := range versions {
		vid := versions[i].ID
		if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeVersion, vid, defaultLoc); err == nil {
			i18n.NewOverlay(fields).Apply("release_notes", &versions[i].ReleaseNotes)
		}
		if loc != defaultLoc {
			if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeVersion, vid, loc); err == nil {
				i18n.NewOverlay(fields).Apply("release_notes", &versions[i].ReleaseNotes)
			}
		}
		versions[i].ReleaseNotes = markdown.ToHTML(versions[i].ReleaseNotes)
		versions[i].FilePath = ""
	}
	return c.JSON(http.StatusOK, versions)
}

func applyProjectOverlay(p *models.Project, fields map[string]string) {
	o := i18n.NewOverlay(fields)
	o.Apply("title", &p.Title)
	o.Apply("tagline", &p.Tagline)
	o.Apply("description", &p.Description)
}

func bestAutoDiscount(discounts []models.AutoDiscount, appID string) *models.AutoDiscount {
	var storeWide *models.AutoDiscount
	for i := range discounts {
		d := &discounts[i]
		if d.AppID != nil && *d.AppID == appID {
			return d
		}
		if d.AppID == nil && storeWide == nil {
			storeWide = d
		}
	}
	return storeWide
}

// ── Sitemap & robots ───────────────────────────────────────────────────────

type sitemapURL struct {
	Loc        string `xml:"loc"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

type sitemapDoc struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// GetSitemap renders /sitemap.xml. Lists the locale-rooted Landing pages and
// every project that has a detail page (commerce or admin-toggled showcase).
func (h *StoreHandler) GetSitemap(c *app.Ctx) error {
	ctx := c.R.Context()
	projects, err := queries.ListProjectsPublic(ctx, c.DB().DB)
	if err != nil {
		h.log.Printf("sitemap: list projects: %v", err)
		http.Error(c.W, "Internal Server Error", http.StatusInternalServerError)
		return nil
	}

	base := strings.TrimRight(h.cfg.BaseURL, "/")

	supported := c.Locales().Supported(ctx)
	defaultLoc := c.Locales().Default(ctx)

	doc := sitemapDoc{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}
	for _, loc := range supported {
		p := "0.9"
		if loc == defaultLoc {
			p = "1.0"
		}
		doc.URLs = append(doc.URLs, sitemapURL{
			Loc: base + "/" + loc + "/", ChangeFreq: "weekly", Priority: p,
		})
	}
	for _, project := range projects {
		if !project.HasDetailPage {
			continue
		}
		slug := project.Slug
		if slug == "" {
			slug = project.ID
		}
		for _, loc := range supported {
			p := "0.8"
			if loc != defaultLoc {
				p = "0.7"
			}
			doc.URLs = append(doc.URLs, sitemapURL{
				Loc:        fmt.Sprintf("%s/%s/project/%s", base, loc, slug),
				ChangeFreq: "weekly",
				Priority:   p,
			})
		}
	}

	c.W.Header().Set("Content-Type", "application/xml; charset=utf-8")
	c.W.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(c.W, xml.Header); err != nil {
		h.log.Printf("sitemap: write xml header: %v", err)
	}
	if err := xml.NewEncoder(c.W).Encode(doc); err != nil {
		h.log.Printf("sitemap: encode: %v", err)
	}
	return nil
}

func (h *StoreHandler) Healthz(c *app.Ctx) error {
	c.W.WriteHeader(http.StatusOK)
	return nil
}

func (h *StoreHandler) GetRobotsTxt(c *app.Ctx) error {
	base := strings.TrimRight(h.cfg.BaseURL, "/")
	c.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.W.Header().Set("Cache-Control", "public, max-age=3600")
	if _, err := fmt.Fprintf(c.W, "User-agent: *\nAllow: /\nDisallow: /admin/\nDisallow: /*/account\nDisallow: /*/login\nDisallow: /*/success\n\nSitemap: %s/sitemap.xml\n", base); err != nil {
		h.log.Printf("robots: write: %v", err)
	}
	return nil
}
