import { DEFAULT_LOCALE } from '../i18n';

const API_BASE = '/api';

let currentLocale = DEFAULT_LOCALE;
export function setClientLocale(locale: string) { currentLocale = locale; }

type MaintenanceListener = () => void;
let onMaintenanceDetected: MaintenanceListener | null = null;
export function setMaintenanceListener(fn: MaintenanceListener | null) {
  onMaintenanceDetected = fn;
}

async function apiFetch<T>(url: string, options: RequestInit | undefined, checkMaintenance: boolean): Promise<T> {
  const res = await fetch(url, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', 'Accept-Language': currentLocale },
    ...options,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: 'unknown', message: res.statusText }));
    if (checkMaintenance && res.status === 503 && body.error === 'maintenance') {
      onMaintenanceDetected?.();
    }
    throw new APIError(body.error, body.message, res.status);
  }

  const ct = res.headers.get('Content-Type') ?? '';
  if (!ct.includes('application/json')) return undefined as T;
  return res.json();
}

// request targets the /api routes and surfaces 503 maintenance responses.
async function request<T>(path: string, options?: RequestInit): Promise<T> {
  return apiFetch<T>(`${API_BASE}${path}`, options, true);
}

// authRequest targets the framework /auth routes (not under /api).
async function authRequest<T>(path: string, options?: RequestInit): Promise<T> {
  return apiFetch<T>(`/auth${path}`, options, false);
}

export class APIError extends Error {
  code: string;
  status: number;

  constructor(code: string, message: string, status: number) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

// Auth (framework routes at /auth/*, not /api/auth/*)
export function sendMagicLink(email: string) {
  return authRequest<{ status: string }>('/login', {
    method: 'POST',
    body: JSON.stringify({ email }),
  });
}

export function verifyToken(token: string) {
  return authRequest<{ user_id: string; role: string }>('/verify', {
    method: 'POST',
    body: JSON.stringify({ token }),
  });
}

export function logout() {
  return authRequest<void>('/logout', { method: 'POST' });
}

export function getMe() {
  return request<{ is_admin: boolean }>('/auth/me');
}

// ── Public catalog (projects + commerce) ───────────────────────────────────

export type PurchaseMode =
  | 'always_new_license'
  | 'one_time_only'
  | 'install_plus'
  | 'coming_soon';

export interface Commerce {
  id: string;
  bundle_id: string;
  price_cents: number;
  discounted_price_cents?: number;
  purchase_mode: PurchaseMode;
  tax_category: string;
}

export interface ProjectImage {
  id: string;
  url: string;
  position: number;
  alt_text?: string;
}

export interface AppVersion {
  id: string;
  app_id: string;
  version: string;
  download_url: string;
  file_path?: string;
  release_notes: string;
  published_at: string;
}

export interface PublicProject {
  id: string;
  slug: string;
  position: number;
  image_url: string;
  external_url?: string | null;
  has_detail_page: boolean;
  title: string;
  tagline: string;
  commerce?: Commerce;
}

export interface ProjectDetail extends PublicProject {
  description?: string;
  images?: ProjectImage[];
  versions?: AppVersion[];
  // Translation overlay from app entity (commerce-only).
  system_requirements?: string;
}

// Backend serializes the project with `commerce` from a JOIN. The detail payload
// adapts for ProjectImage/AppVersion subfields. The wire shape matches models.Project.
function normalizeProject<T extends { commerce?: Commerce | null }>(p: T): T {
  // Backend may emit commerce: null when the JOIN returns no row; collapse to undefined.
  if (p && p.commerce == null) {
    delete (p as { commerce?: Commerce | null }).commerce;
  }
  return p;
}

export function getProjects(): Promise<PublicProject[]> {
  return request<PublicProject[]>('/projects').then(arr => arr.map(normalizeProject));
}

export function getProjectDetail(slug: string): Promise<ProjectDetail> {
  return request<ProjectDetail>(`/projects/${encodeURIComponent(slug)}`).then(p => {
    normalizeProject(p);
    // Lift system_requirements from the nested commerce overlay (backend already
    // applies it; surface it on the top-level detail object for convenience).
    if (p.commerce && (p.commerce as Commerce & { system_requirements?: string }).system_requirements) {
      p.system_requirements = (p.commerce as Commerce & { system_requirements?: string }).system_requirements;
    }
    return p;
  });
}

export function getProjectVersions(slug: string): Promise<AppVersion[]> {
  return request<AppVersion[]>(`/projects/${encodeURIComponent(slug)}/versions`);
}

// ── Ownership (still keyed by commerce app id) ─────────────────────────────

export interface OwnershipStatus {
  has_license: boolean;
  purchase_mode: PurchaseMode;
}

export function getOwnership(appId: string): Promise<OwnershipStatus> {
  return request<OwnershipStatus>(`/apps/${encodeURIComponent(appId)}/ownership`);
}

// ── Account ────────────────────────────────────────────────────────────────

export interface Activation {
  id: string;
  license_id: string;
  machine_hash: string;
  device_label: string | null;
  activated_at: string;
  last_seen_at: string | null;
}

export interface License {
  id: string;
  key: string;
  order_id: string;
  app_id: string;
  app_bundle_id: string;
  app_name: string;
  max_activations?: number;
  created_at: string;
  activations: Activation[];
}

export function getLicenses() {
  return request<License[]>('/account/licenses');
}

export function getOrders() {
  return request<UserOrder[]>('/account/orders');
}

export function renameDevice(activationId: string, deviceLabel: string) {
  return request<{ message: string }>(`/account/activations/${activationId}`, {
    method: 'PATCH',
    body: JSON.stringify({ device_label: deviceLabel }),
  });
}

export function createDownloadToken(licenseId: string) {
  return request<{ url: string; expires_at: string }>(
    `/account/licenses/${licenseId}/download-token`,
    { method: 'POST' }
  );
}

// ── Admin — stats ──────────────────────────────────────────────────────────

export interface AdminStats {
  total_revenue_cents: number;
  revenue_30d_cents: number;
  total_orders: number;
  total_licenses: number;
  total_activations: number;
}

export async function adminGetStats(): Promise<AdminStats> {
  return request<AdminStats>('/admin/stats');
}

// ── Admin — apps (commerce attachments) ────────────────────────────────────

// AdminAppListItem mirrors backend queries.AdminAppListItem (commerce row +
// joined project_slug/project_title). Used by admin list views (orders/licenses
// dropdowns) where the "name" of the app comes from its parent project.
export interface AdminAppListItem {
  id: string;
  project_id: string;
  bundle_id: string;
  price_cents: number;
  purchase_mode: PurchaseMode;
  tax_category: string;
  created_at: string;
  deleted_at?: string | null;
  project_slug: string;
  project_title?: string;
}

export async function adminListApps(): Promise<AdminAppListItem[]> {
  return request<AdminAppListItem[]>('/admin/apps');
}

// AdminApp is the full commerce row returned by attach/create endpoints.
export interface AdminApp {
  id: string;
  project_id: string;
  bundle_id: string;
  price_cents: number;
  purchase_mode: PurchaseMode;
  tax_category: string;
  discounted_price_cents?: number;
  system_requirements?: string;
  created_at: string;
  deleted_at?: string | null;
}

export function adminUpdateApp(
  id: string,
  data: { price_cents: number; purchase_mode: string; tax_category: string }
): Promise<void> {
  return request<void>(`/admin/apps/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export function adminListVersions(appId: string) {
  return request<AppVersion[]>(`/admin/apps/${appId}/versions`);
}

export function adminCreateVersion(
  appId: string,
  data: { version: string; release_notes: string }
) {
  return request<AppVersion>(`/admin/apps/${appId}/versions`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

// ── Account — data export and erasure ──────────────────────────────────────

async function downloadBlob(path: string, filename: string): Promise<void> {
  const res = await fetch(`${API_BASE}${path}`, { credentials: 'include' });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: 'unknown', message: res.statusText }));
    throw new APIError(body.error, body.message, res.status);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export function exportUserData(): Promise<void> {
  return downloadBlob('/account/data', 'account-data.json');
}

export function deleteUserData(email: string): Promise<void> {
  return request<void>('/account/data', {
    method: 'DELETE',
    body: JSON.stringify({ email }),
  });
}

// ── Admin — user management ────────────────────────────────────────────────

export interface UserSession {
  id: string;
  created_at: string;
  expires_at: string;
}

export interface UserOrder {
  id: string;
  app_name: string;
  app_id: string;
  price_paid_cents: number;
  original_price_cents?: number;
  discount_label?: string;
  discount_type?: string;
  discount_value?: number;
  created_at: string;
}

export interface UserDownloadToken {
  token: string;
  app_id: string;
  expires_at: string;
  used: boolean;
  created_at: string;
}

export interface AdminUserData {
  hash: string;
  sessions: UserSession[];
  orders: UserOrder[];
  licenses: License[];
  activations: Activation[];
  download_tokens: UserDownloadToken[];
}

export function adminLookupUser(email: string): Promise<AdminUserData> {
  return request<AdminUserData>('/admin/users/lookup', {
    method: 'POST',
    body: JSON.stringify({ email }),
  });
}

export function adminRenameActivation(activationId: string, deviceLabel: string): Promise<{ message: string }> {
  return request<{ message: string }>(`/admin/activations/${activationId}`, {
    method: 'PATCH',
    body: JSON.stringify({ device_label: deviceLabel }),
  });
}

export function adminRevokeActivation(activationId: string): Promise<void> {
  return request<void>(`/admin/activations/${activationId}`, { method: 'DELETE' });
}

export function adminDeleteUserSessions(hash: string): Promise<{ deleted: number }> {
  return request<{ deleted: number }>(`/admin/users/${hash}/sessions`, { method: 'DELETE' });
}

export function adminVoidOrder(orderId: string): Promise<void> {
  return request<void>(`/admin/orders/${orderId}`, { method: 'DELETE' });
}

// ── Admin — orders list ────────────────────────────────────────────────────

export interface AdminOrderListResult {
  orders: AdminOrder[];
  total: number;
}
export interface AdminOrder {
  id: string;
  payment_session: string;
  email: string;
  app_id: string;
  app_name: string;
  price_paid_cents: number;
  original_price_cents?: number;
  discount_label?: string;
  created_at: string;
}

export async function adminListOrders(params: Record<string, string>): Promise<AdminOrderListResult> {
  const qs = new URLSearchParams(params).toString();
  return request<AdminOrderListResult>(`/admin/orders?${qs}`);
}

// ── Admin — licenses list ──────────────────────────────────────────────────

export interface AdminLicenseListResult {
    licenses: AdminLicense[];
    total: number;
}
export interface AdminLicense {
    id: string;
    key: string;
    order_id: string;
    app_id: string;
    app_name: string;
    revoked: boolean;
    max_activations?: number;
    activation_count: number;
    created_at: string;
}

export async function adminListLicenses(params: Record<string, string>): Promise<AdminLicenseListResult> {
    const qs = new URLSearchParams(params).toString();
    return request<AdminLicenseListResult>(`/admin/licenses?${qs}`);
}

export async function adminIssueLicense(data: { email: string; app_id: string; price_cents: number }): Promise<{ license_key: string }> {
    return request<{ license_key: string }>('/admin/licenses', {
        method: 'POST',
        body: JSON.stringify(data),
    });
}

export async function adminUnrevokeLicense(id: string): Promise<void> {
    await request<void>(`/admin/licenses/${id}/unrevoke`, { method: 'PATCH' });
}

// ── Admin — sales dashboard ────────────────────────────────────────────────

export interface AppSalesRow {
  app_id: string;
  app_name: string;
  order_count: number;
  revenue_cents: number;
}

export interface SalesReport {
  rows: AppSalesRow[];
  total_orders: number;
  total_revenue_cents: number;
}

export function adminGetSales(start?: string, end?: string): Promise<SalesReport> {
  const params = new URLSearchParams();
  if (start) params.set('start', start);
  if (end) params.set('end', end);
  const qs = params.toString();
  return request<SalesReport>(`/admin/sales${qs ? '?' + qs : ''}`);
}

// ── Discount codes — public ────────────────────────────────────────────────

export interface DiscountValidation {
  discount_type: 'percent' | 'fixed';
  discount_value: number;
  original_price_cents: number;
  final_price_cents: number;
  stacked_with_auto?: boolean;
}

export function validateDiscountCode(code: string, appId: string): Promise<DiscountValidation> {
  return request<DiscountValidation>('/discounts/validate', {
    method: 'POST',
    body: JSON.stringify({ code, app_id: appId }),
  });
}

export function getAutoDiscount(appId: string): Promise<DiscountValidation> {
  return request<DiscountValidation>(`/discounts/auto?app_id=${encodeURIComponent(appId)}`);
}

// ── Discount codes — admin ─────────────────────────────────────────────────

export interface DiscountCode {
  id: string;
  code: string;
  label: string;
  discount_type: 'percent' | 'fixed';
  discount_value: number;
  app_id: string | null;
  max_uses: number | null;
  uses: number;
  expires_at: string | null;
  active: boolean;
  stackable: boolean;
  created_at: string;
  deleted_at: string | null;
  // Stats from joined orders
  order_count: number;
  revenue_cents: number;
}

export function adminListDiscountCodes(): Promise<DiscountCode[]> {
  return request<DiscountCode[]>('/admin/discount-codes');
}

export function adminCreateDiscountCode(data: {
  code: string;
  label: string;
  discount_type: 'percent' | 'fixed';
  discount_value: number;
  app_id: string | null;
  max_uses: number | null;
  expires_at: string | null;
  stackable: boolean;
}): Promise<DiscountCode> {
  return request<DiscountCode>('/admin/discount-codes', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function adminUpdateDiscountCode(
  id: string,
  data: {
    label: string;
    discount_type: 'percent' | 'fixed';
    discount_value: number;
    app_id: string | null;
    max_uses: number | null;
    expires_at: string | null;
    active: boolean;
    stackable: boolean;
  }
): Promise<DiscountCode> {
  return request<DiscountCode>(`/admin/discount-codes/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export function adminDeleteDiscountCode(id: string): Promise<void> {
  return request<void>(`/admin/discount-codes/${id}`, { method: 'DELETE' });
}

export function adminRestoreDiscountCode(id: string): Promise<void> {
  return request<void>(`/admin/discount-codes/${id}/restore`, { method: 'PATCH' });
}

// ── Auto discounts — admin ─────────────────────────────────────────────────

export interface AutoDiscount {
  id: string;
  label: string;
  discount_type: 'percent' | 'fixed';
  discount_value: number;
  app_id: string | null;
  valid_from: string | null;
  expires_at: string | null;
  active: boolean;
  created_at: string;
  deleted_at: string | null;
  // Stats from joined orders
  order_count: number;
  revenue_cents: number;
}

export function adminListAutoDiscounts(): Promise<AutoDiscount[]> {
  return request<AutoDiscount[]>('/admin/auto-discounts');
}

export function adminCreateAutoDiscount(data: {
  label: string;
  discount_type: 'percent' | 'fixed';
  discount_value: number;
  app_id: string | null;
  valid_from: string | null;
  expires_at: string | null;
}): Promise<AutoDiscount> {
  return request<AutoDiscount>('/admin/auto-discounts', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function adminUpdateAutoDiscount(
  id: string,
  data: {
    label: string;
    discount_type: 'percent' | 'fixed';
    discount_value: number;
    app_id: string | null;
    valid_from: string | null;
    expires_at: string | null;
    active: boolean;
  }
): Promise<AutoDiscount> {
  return request<AutoDiscount>(`/admin/auto-discounts/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export function adminDeleteAutoDiscount(id: string): Promise<void> {
  return request<void>(`/admin/auto-discounts/${id}`, { method: 'DELETE' });
}

export function adminRestoreAutoDiscount(id: string): Promise<void> {
  return request<void>(`/admin/auto-discounts/${id}/restore`, { method: 'PATCH' });
}

// ── Checkout ───────────────────────────────────────────────────────────────

export interface CheckoutSession {
  url: string;            // mock provider: redirect target
  transaction_id: string; // Paddle: txn_… id for the client-side overlay
  session_id: string;     // idempotency key; passed to the success page
}

export function createCheckoutSession(
  appId: string,
  discountCode: string,
  consentTimestamp: string,
): Promise<CheckoutSession> {
  return request<CheckoutSession>('/checkout/session', {
    method: 'POST',
    body: JSON.stringify({
      app_id: appId,
      discount_code: discountCode,
      consent_timestamp: consentTimestamp,
    }),
  });
}

export interface CheckoutVerification {
  license_key: string;
  app_name: string;
  bundle_id: string;
}

export function verifyCheckout(sessionId: string): Promise<CheckoutVerification> {
  return request<CheckoutVerification>(`/checkout/verify?session_id=${encodeURIComponent(sessionId)}`);
}

// ── Site config — public ───────────────────────────────────────────────────
//
// Boolean flags (maintenance_mode, payment_enabled) and max_activations are
// already coerced to proper JSON types by the backend's GetPublicConfig.
// Don't reintroduce string parsing on the consumer side.

export interface SiteConfig {
  currency_symbol: string;
  currency_code: string;
  site_name: string;
  maintenance_mode: boolean;
  payment_enabled: boolean;
  payment_provider: string;
  paddle_client_token: string;
  paddle_environment: string;
  max_activations: number;
  base_url: string;
  locales: Array<{ code: string; name: string; is_default: boolean }>;
}

export function getPublicConfig(): Promise<SiteConfig> {
  return request<SiteConfig>('/config');
}

// ── Legal pages — public ───────────────────────────────────────────────────

export function getLegalPage(slug: 'impressum' | 'privacy' | 'refund-policy'): Promise<{ html: string }> {
  return request<{ html: string }>(`/legal/${slug}`);
}

// ── Project translations — admin ───────────────────────────────────────────

export interface ProjectTranslation {
  title: string;
  tagline: string;
  description: string;
}

export function adminGetProjectTranslations(projectId: string, locale: string): Promise<ProjectTranslation> {
  return request<ProjectTranslation>(`/admin/projects/${projectId}/translations/${locale}`);
}

export function adminUpsertProjectTranslation(projectId: string, locale: string, data: ProjectTranslation): Promise<void> {
  return request<void>(`/admin/projects/${projectId}/translations/${locale}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

// ── App (commerce) translations — admin: only system_requirements ──────────

interface AppTranslationData {
  system_requirements: string;
}

export function adminGetAppTranslations(appId: string): Promise<Record<string, AppTranslationData>> {
  return request<Record<string, AppTranslationData>>(`/admin/apps/${appId}/translations`);
}

export function adminUpsertAppTranslation(appId: string, locale: string, data: AppTranslationData): Promise<void> {
  return request<void>(`/admin/apps/${appId}/translations/${locale}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

// ── Project image (gallery) translations — admin ───────────────────────────

export function adminGetProjectImageTranslations(projectId: string): Promise<Record<string, Record<string, { alt_text: string }>>> {
  return request<Record<string, Record<string, { alt_text: string }>>>(`/admin/projects/${projectId}/images/translations`);
}

export function adminUpsertProjectImageTranslation(projectId: string, imageId: string, locale: string, data: { alt_text: string }): Promise<void> {
  return request<void>(`/admin/projects/${projectId}/images/${imageId}/translations/${locale}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

// ── Version translations — admin ───────────────────────────────────────────

export function adminGetVersionTranslations(appId: string): Promise<Record<string, Record<string, { release_notes: string }>>> {
  return request<Record<string, Record<string, { release_notes: string }>>>(`/admin/apps/${appId}/versions/translations`);
}

export function adminUpsertVersionTranslation(appId: string, versionId: string, locale: string, data: { release_notes: string }): Promise<void> {
  return request<void>(`/admin/apps/${appId}/versions/${versionId}/translations/${locale}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

// ── Shop settings — admin ──────────────────────────────────────────────────

export function adminGetSettings(): Promise<Record<string, string>> {
  return request<Record<string, string>>('/admin/settings');
}

export function adminUpdateSetting(key: string, value: string): Promise<void> {
  return request<void>('/admin/settings', {
    method: 'PATCH',
    body: JSON.stringify({ key, value }),
  });
}

// ── Admin — locales ────────────────────────────────────────────────────────

export interface AdminLocale {
  code: string;
  name: string;
  is_default: boolean;
  enabled: boolean;
  sort_order: number;
}

export function adminListLocales(): Promise<AdminLocale[]> {
  return request<AdminLocale[]>('/admin/locales');
}

export function adminCreateLocale(data: { code: string; name: string; sort_order: number }): Promise<AdminLocale> {
  return request<AdminLocale>('/admin/locales', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function adminUpdateLocale(code: string, data: { enabled?: boolean; is_default?: boolean }): Promise<void> {
  return request<void>(`/admin/locales/${code}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

// ── Admin — page translations ──────────────────────────────────────────────

export function adminGetPageTranslation(pageKey: string, locale: string): Promise<Record<string, string>> {
  return request<Record<string, string>>(`/admin/page-translations/${pageKey}/${locale}`);
}

export function adminUpsertPageTranslation(pageKey: string, locale: string, fields: Record<string, string>): Promise<void> {
  return request<void>(`/admin/page-translations/${pageKey}/${locale}`, {
    method: 'PUT',
    body: JSON.stringify({ fields }),
  });
}

// ── Admin — mail templates ─────────────────────────────────────────────────

export interface MailTemplate {
    subject: string;
    body: string;
}

export async function adminGetMailTemplate(locale: string): Promise<MailTemplate> {
    return request<MailTemplate>(`/admin/mail-templates/${locale}`);
}

export async function adminUpsertMailTemplate(locale: string, data: MailTemplate): Promise<void> {
    await request<void>(`/admin/mail-templates/${locale}`, {
        method: 'PUT',
        body: JSON.stringify(data),
    });
}

// ── Chat — customer ────────────────────────────────────────────────────────

export interface ChatConversation {
  id: string;
  display_name: string;
  email_shared: boolean;
  created_at: string;
  updated_at: string;
}

export interface ChatMessage {
  id: string;
  conversation_id: string;
  sender: 'user' | 'admin';
  body: string;
  created_at: string;
}

export interface ChatResponse {
  conversation: ChatConversation;
  messages: ChatMessage[];
  has_unread: boolean;
}

export function getChat(): Promise<ChatResponse> {
  return request<ChatResponse>('/chat');
}

export function createChat(): Promise<ChatConversation> {
  return request<ChatConversation>('/chat', { method: 'POST' });
}

export function sendChatMessage(body: string): Promise<ChatMessage> {
  return request<ChatMessage>('/chat/messages', {
    method: 'POST',
    body: JSON.stringify({ body }),
  });
}

export function shareChatEmail(email: string): Promise<{ status: string }> {
  return request<{ status: string }>('/chat/share-email', {
    method: 'POST',
    body: JSON.stringify({ email }),
  });
}

export function deleteChat(): Promise<void> {
  return request<void>('/chat', { method: 'DELETE' });
}

// ── Chat — admin ───────────────────────────────────────────────────────────

export interface AdminChatListItem {
  id: string;
  display_name: string;
  email: string | null;
  has_unread: boolean;
  last_message_preview: string;
  message_count: number;
  created_at: string;
  updated_at: string;
}

export interface AdminChatListResult {
  items: AdminChatListItem[];
  total: number;
}

export interface AdminChatConversation {
  id: string;
  user_id: string;
  display_name: string;
  email: string | null;
  created_at: string;
  updated_at: string;
}

export interface AdminChatDetailResponse {
  conversation: AdminChatConversation;
  messages: ChatMessage[];
  has_unread: boolean;
  is_banned: boolean;
}

export function adminListChats(params: Record<string, string>): Promise<AdminChatListResult> {
  const qs = new URLSearchParams(params).toString();
  return request<AdminChatListResult>(`/admin/chats?${qs}`);
}

export function adminGetChat(id: string): Promise<AdminChatDetailResponse> {
  return request<AdminChatDetailResponse>(`/admin/chats/${id}`);
}

export function adminSendChatMessage(id: string, body: string): Promise<ChatMessage> {
  return request<ChatMessage>(`/admin/chats/${id}/messages`, {
    method: 'POST',
    body: JSON.stringify({ body }),
  });
}

export function adminDeleteChat(id: string): Promise<void> {
  return request<void>(`/admin/chats/${id}`, { method: 'DELETE' });
}

export function adminBanChatUser(id: string, reason: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/admin/chats/${id}/ban`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  });
}

export function adminUnbanChatUser(id: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/admin/chats/${id}/unban`, { method: 'POST' });
}

export function adminChatUnreadCount(): Promise<{ count: number }> {
  return request<{ count: number }>('/admin/chats/unread-count');
}

// ── Admin — multipart file uploads (bypass the JSON helper) ────────────────

async function uploadFile(url: string, file: File, extraFields?: Record<string, string>) {
  const form = new FormData();
  form.append('file', file);
  if (extraFields) {
    for (const [k, v] of Object.entries(extraFields)) {
      form.append(k, v);
    }
  }
  const res = await fetch(`${API_BASE}${url}`, {
    method: 'POST',
    credentials: 'include',
    body: form,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: 'unknown', message: res.statusText }));
    throw new APIError(body.error, body.message, res.status);
  }
  return res.json();
}

export function adminUploadBinary(
  appId: string,
  versionId: string,
  file: File
): Promise<{ file_path: string }> {
  return uploadFile(`/admin/apps/${appId}/versions/${versionId}/upload`, file);
}

// ── Admin — Signing Keys ───────────────────────────────────────────────────

export interface SigningKey {
  id: string;
  key_id: string;
  public_key_b64: string;
  active: boolean;
  created_at: string;
}

export function adminGetSigningKey(): Promise<{ key: SigningKey | null }> {
  return request<{ key: SigningKey | null }>('/admin/signing-keys');
}

export function adminGenerateSigningKey(): Promise<SigningKey> {
  return request<SigningKey>('/admin/signing-keys', { method: 'POST' });
}

// ── Admin — Projects ───────────────────────────────────────────────────────

export interface AdminProject extends PublicProject {
  description?: string;
  deleted_at?: string | null;
  created_at: string;
}

// CommerceInput is the optional commerce attachment payload included with
// AdminCreateProject. Empty bundle_id means "no commerce".
export interface CommerceInput {
  bundle_id: string;
  price_cents: number;
  purchase_mode: string;
  tax_category: string;
}

export interface CreateProjectInput {
  slug?: string;
  external_url?: string | null;
  image_url?: string;
  has_detail_page?: boolean;
  title: string;
  tagline: string;
  description: string;
  commerce?: CommerceInput;
}

export interface UpdateProjectInput {
  slug?: string;
  external_url?: string | null;
  image_url?: string;
  has_detail_page?: boolean;
  title: string;
  tagline: string;
  description: string;
}

export function adminListProjects(): Promise<AdminProject[]> {
  return request<AdminProject[]>('/admin/projects').then(arr => arr.map(normalizeProject));
}

export function adminCreateProject(data: CreateProjectInput): Promise<AdminProject> {
  return request<AdminProject>('/admin/projects', {
    method: 'POST',
    body: JSON.stringify(data),
  }).then(normalizeProject);
}

export function adminUpdateProject(id: string, data: UpdateProjectInput): Promise<void> {
  return request<void>(`/admin/projects/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export function adminDeleteProject(id: string): Promise<void> {
  return request<void>(`/admin/projects/${id}`, { method: 'DELETE' });
}

export function adminRestoreProject(id: string): Promise<void> {
  return request<void>(`/admin/projects/${id}/restore`, { method: 'POST' });
}

export function adminReorderProjects(positions: Record<string, number>): Promise<void> {
  return request<void>('/admin/projects/reorder', {
    method: 'PATCH',
    body: JSON.stringify({ positions }),
  });
}

export function adminUploadProjectImage(projectId: string, file: File): Promise<{ image_url: string }> {
  return uploadFile(`/admin/projects/${projectId}/image`, file);
}

// Commerce attach/detach (manages the apps row attached to a project).
export function adminAttachCommerce(projectId: string, data: CommerceInput): Promise<AdminApp> {
  return request<AdminApp>(`/admin/projects/${projectId}/commerce`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function adminDetachCommerce(projectId: string): Promise<void> {
  return request<void>(`/admin/projects/${projectId}/commerce`, { method: 'DELETE' });
}

// Project gallery images (replaces app screenshots).
export function adminUploadProjectGalleryImage(
  projectId: string,
  file: File,
  altText: string
): Promise<ProjectImage> {
  return uploadFile(`/admin/projects/${projectId}/images`, file, { alt_text: altText });
}

export function adminReorderProjectImages(projectId: string, positions: Record<string, number>): Promise<void> {
  return request<void>(`/admin/projects/${projectId}/images/reorder`, {
    method: 'PATCH',
    body: JSON.stringify({ positions }),
  });
}

export function adminDeleteProjectImage(projectId: string, imageId: string): Promise<void> {
  return request<void>(`/admin/projects/${projectId}/images/${imageId}`, {
    method: 'DELETE',
  });
}

// ── Admin — Social Links ───────────────────────────────────────────────────

export interface AdminSocialLink {
  id: string;
  platform: string;
  url: string;
  position: number;
  created_at: string;
}

export function adminListSocialLinks(): Promise<AdminSocialLink[]> {
  return request<AdminSocialLink[]>('/admin/social-links');
}

export function adminCreateSocialLink(data: { platform: string; url: string }): Promise<AdminSocialLink> {
  return request<AdminSocialLink>('/admin/social-links', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function adminUpdateSocialLink(id: string, data: { platform: string; url: string }): Promise<void> {
  return request<void>(`/admin/social-links/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export function adminDeleteSocialLink(id: string): Promise<void> {
  return request<void>(`/admin/social-links/${id}`, { method: 'DELETE' });
}

export function adminReorderSocialLinks(positions: Record<string, number>): Promise<void> {
  return request<void>('/admin/social-links/reorder', {
    method: 'PATCH',
    body: JSON.stringify({ positions }),
  });
}

// ── Public: Hero, Social Links ─────────────────────────────────────────────

export interface HeroContent {
  headline: string;
  tagline: string;
  bio: string;
}

export interface PublicSocialLink {
  id: string;
  platform: string;
  url: string;
  position: number;
}

export function getHero(): Promise<HeroContent> {
  return request<HeroContent>('/hero');
}

export function listPublicSocialLinks(): Promise<PublicSocialLink[]> {
  return request<PublicSocialLink[]>('/social-links');
}
