package handlers

import (
	appsvc "github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/middleware"
)

// RegisterRoutes registers all application routes on the given appsvc.App.
// The maintenance middleware is applied to public routes that should be blocked
// during maintenance mode. Pass nil for no-op maintenance (e.g. tests).
func RegisterRoutes(app *appsvc.App, h *Handlers, maintenance middleware.Middleware) {
	api := app.Group("/api")

	// Maintenance-aware scopes: block all public & authenticated routes during
	// maintenance. When maintenance is nil (tests), these are identical to the
	// bare scopes.
	pubM := api.Public()
	authM := api.Authenticated()
	if maintenance != nil {
		pubM = api.Public().With(maintenance)
		authM = api.Authenticated().With(maintenance)
	}

	// --- Bare (top-level, no /api prefix) ---

	app.Public().Handle("GET /healthz", h.Store.Healthz)
	app.Public().Handle("GET /sitemap.xml", h.Store.GetSitemap)
	app.Public().Handle("GET /robots.txt", h.Store.GetRobotsTxt)
	app.Public().Handle("POST /api/payment/webhook", h.Checkout.WebhookReceive)

	// --- Public (maintenance-exempt) ---

	api.Public().Handle("GET /config", h.Store.GetPublicConfig)
	api.Public().Handle("GET /legal/impressum", h.Store.GetImpressum)
	api.Public().Handle("GET /legal/privacy", h.Store.GetPrivacyPolicy)
	api.Public().Handle("GET /legal/refund-policy", h.Store.GetRefundPolicy)
	api.Public().Handle("GET /checkout/verify", h.Checkout.VerifyCheckout)

	// --- Public (maintenance-blocked) ---

	pubM.Handle("GET /projects", h.Store.ListProjects)
	pubM.Handle("GET /projects/{slug}", h.Store.GetProjectDetail)
	pubM.Handle("GET /projects/{slug}/versions", h.Store.GetProjectVersions)

	pubM.Handle("GET /social-links", h.Store.ListSocialLinks)
	pubM.Handle("GET /hero", h.Store.GetHero)

	pubM.Handle("POST /license/activate", h.SDK.ActivateLicense)
	pubM.Handle("POST /license/deactivate", h.SDK.DeactivateLicense)
	pubM.Handle("GET /updates/check", h.SDK.CheckForUpdate)
	pubM.Handle("GET /downloads/file", h.SDK.ServeDownload)

	pubM.Handle("POST /discounts/validate", h.Discounts.ValidateDiscount, appsvc.WithRateLimit(app, 3, h.TrustedProxy))
	pubM.Handle("GET /discounts/auto", h.Discounts.GetAutoDiscount)

	pubM.Handle("POST /checkout/session", h.Checkout.CreateCheckout)
	pubM.Handle("GET /checkout/mock/trigger", h.Checkout.MockComplete)

	// --- Authenticated (maintenance-blocked) ---

	authM.Handle("GET /auth/me", h.Account.GetMe)
	authM.Handle("GET /account/licenses", h.Account.GetLicenses)
	authM.Handle("GET /account/orders", h.Account.GetOrders)
	authM.Handle("PATCH /account/activations/{id}", h.Account.RenameDevice)
	authM.Handle("POST /account/licenses/{licenseId}/download-token", h.Account.CreateDownloadToken)
	authM.Handle("GET /account/data", h.Account.ExportUserData)
	authM.Handle("DELETE /account/data", h.Account.DeleteUserData)
	authM.Handle("GET /apps/{id}/ownership", h.Account.GetOwnership)

	// --- Chat (authenticated, maintenance-blocked) ---

	authM.Handle("GET /chat", h.Chat.GetChat)
	authM.Handle("POST /chat", h.Chat.CreateChat)
	authM.Handle("POST /chat/messages", h.Chat.SendMessage, appsvc.WithRateLimit(app, 60, h.TrustedProxy))
	authM.Handle("POST /chat/share-email", h.Chat.ShareEmail)
	authM.Handle("DELETE /chat", h.Chat.DeleteChat)

	// --- Admin (role-restricted) ---

	adm := api.Role("admin")

	// Apps (commerce attachment — admin-only; public access goes through projects)
	adm.Handle("GET /admin/apps", h.Admin.AdminListApps)
	adm.Handle("POST /admin/apps", h.Admin.AdminCreateApp)
	adm.Handle("PATCH /admin/apps/{id}", h.Admin.AdminUpdateApp)
	adm.Handle("DELETE /admin/apps/{id}", h.Admin.AdminDeleteApp)
	adm.Handle("POST /admin/apps/{id}/restore", h.Admin.AdminRestoreApp)
	adm.Handle("GET /admin/apps/{id}/versions", h.Admin.AdminListVersions)
	adm.Handle("POST /admin/apps/{id}/versions", h.Admin.AdminCreateVersion)
	adm.Handle("POST /admin/apps/{id}/versions/{vid}/upload", h.Admin.AdminUploadBinary)
	adm.Handle("POST /admin/users/lookup", h.Admin.AdminLookupUser)
	adm.Handle("PATCH /admin/activations/{id}", h.Admin.AdminRenameActivation)
	adm.Handle("DELETE /admin/activations/{id}", h.Admin.AdminRevokeActivation)
	adm.Handle("DELETE /admin/users/{hash}/sessions", h.Admin.AdminDeleteUserSessions)
	adm.Handle("DELETE /admin/orders/{id}", h.Admin.AdminVoidOrder)
	adm.Handle("GET /admin/orders", h.Admin.AdminListOrders)
	adm.Handle("GET /admin/stats", h.Admin.AdminGetStats)
	adm.Handle("GET /admin/licenses", h.Admin.AdminListLicenses)
	adm.Handle("POST /admin/licenses", h.Admin.AdminIssueLicense)
	adm.Handle("PATCH /admin/licenses/{id}/unrevoke", h.Admin.AdminUnrevokeLicense)
	adm.Handle("GET /admin/sales", h.Admin.AdminGetSales)
	adm.Handle("GET /admin/settings", h.Admin.AdminGetSettings)
	adm.Handle("PATCH /admin/settings", h.Admin.AdminUpdateSetting)
	adm.Handle("GET /admin/discount-codes", h.Admin.AdminListDiscountCodes)
	adm.Handle("POST /admin/discount-codes", h.Admin.AdminCreateDiscountCode)
	adm.Handle("PATCH /admin/discount-codes/{id}", h.Admin.AdminUpdateDiscountCode)
	adm.Handle("DELETE /admin/discount-codes/{id}", h.Admin.AdminDeleteDiscountCode)
	adm.Handle("PATCH /admin/discount-codes/{id}/restore", h.Admin.AdminRestoreDiscountCode)
	adm.Handle("GET /admin/auto-discounts", h.Admin.AdminListAutoDiscounts)
	adm.Handle("POST /admin/auto-discounts", h.Admin.AdminCreateAutoDiscount)
	adm.Handle("PATCH /admin/auto-discounts/{id}", h.Admin.AdminUpdateAutoDiscount)
	adm.Handle("DELETE /admin/auto-discounts/{id}", h.Admin.AdminDeleteAutoDiscount)
	adm.Handle("PATCH /admin/auto-discounts/{id}/restore", h.Admin.AdminRestoreAutoDiscount)

	// App translations (commerce-only: system_requirements, version release_notes)
	adm.Handle("GET /admin/apps/{id}/translations", h.Admin.AdminGetAppTranslations)
	adm.Handle("PUT /admin/apps/{id}/translations/{locale}", h.Admin.AdminUpsertAppTranslation)
	adm.Handle("GET /admin/apps/{id}/versions/translations", h.Admin.AdminGetVersionTranslations)
	adm.Handle("PUT /admin/apps/{id}/versions/{vid}/translations/{locale}", h.Admin.AdminUpsertVersionTranslation)

	// Locale management
	api.Public().Handle("GET /i18n/{locale}", h.Store.GetUITranslations)

	adm.Handle("GET /admin/locales", h.Admin.AdminListLocales)
	adm.Handle("POST /admin/locales", h.Admin.AdminCreateLocale)
	adm.Handle("PATCH /admin/locales/{code}", h.Admin.AdminUpdateLocale)

	adm.Handle("GET /admin/i18n/{locale}", h.Admin.AdminGetUITranslations)
	adm.Handle("PUT /admin/i18n/{locale}", h.Admin.AdminUpsertUITranslation)
	adm.Handle("DELETE /admin/i18n/{locale}/{key}", h.Admin.AdminDeleteUITranslation)

	adm.Handle("GET /admin/mail-templates/{locale}", h.Admin.AdminGetMailTemplate)
	adm.Handle("PUT /admin/mail-templates/{locale}", h.Admin.AdminUpsertMailTemplate)

	// Page translations (i18n)
	adm.Handle("GET /admin/page-translations", h.Admin.AdminListPageTranslations)
	adm.Handle("GET /admin/page-translations/{pageKey}/{locale}", h.Admin.AdminGetPageTranslation)
	adm.Handle("PUT /admin/page-translations/{pageKey}/{locale}", h.Admin.AdminUpsertPageTranslation)
	adm.Handle("DELETE /admin/page-translations/{pageKey}/{locale}", h.Admin.AdminDeletePageTranslation)

	// Signing keys
	adm.Handle("GET /admin/signing-keys", h.Admin.AdminGetSigningKey)
	adm.Handle("POST /admin/signing-keys", h.Admin.AdminGenerateSigningKey)
	adm.Handle("GET /admin/signing-keys/public-key", h.Admin.AdminExportPublicKey)

	// Projects (canonical content entity)
	adm.Handle("GET /admin/projects", h.Admin.AdminListProjects)
	adm.Handle("POST /admin/projects", h.Admin.AdminCreateProject)
	adm.Handle("PATCH /admin/projects/{id}", h.Admin.AdminUpdateProject)
	adm.Handle("DELETE /admin/projects/{id}", h.Admin.AdminDeleteProject)
	adm.Handle("POST /admin/projects/{id}/restore", h.Admin.AdminRestoreProject)
	adm.Handle("POST /admin/projects/{id}/image", h.Admin.AdminUploadProjectImage)
	adm.Handle("PATCH /admin/projects/reorder", h.Admin.AdminReorderProjects)
	adm.Handle("GET /admin/projects/{id}/translations/{locale}", h.Admin.AdminGetProjectTranslations)
	adm.Handle("PUT /admin/projects/{id}/translations/{locale}", h.Admin.AdminUpsertProjectTranslation)

	// Project gallery images (replaces app screenshots)
	adm.Handle("POST /admin/projects/{id}/images", h.Admin.AdminUploadProjectGalleryImage)
	adm.Handle("PATCH /admin/projects/{id}/images/reorder", h.Admin.AdminReorderProjectImages)
	adm.Handle("DELETE /admin/projects/{id}/images/{imgId}", h.Admin.AdminDeleteProjectImage)
	adm.Handle("GET /admin/projects/{id}/images/translations", h.Admin.AdminGetProjectImageTranslations)
	adm.Handle("PUT /admin/projects/{id}/images/{imgId}/translations/{locale}", h.Admin.AdminUpsertProjectImageTranslation)

	// Commerce attach/detach (creates/removes the apps row attached to a project)
	adm.Handle("POST /admin/projects/{id}/commerce", h.Admin.AdminAttachCommerce)
	adm.Handle("DELETE /admin/projects/{id}/commerce", h.Admin.AdminDetachCommerce)

	// Social Links
	adm.Handle("GET /admin/social-links", h.Admin.AdminListSocialLinks)
	adm.Handle("POST /admin/social-links", h.Admin.AdminCreateSocialLink)
	adm.Handle("PATCH /admin/social-links/{id}", h.Admin.AdminUpdateSocialLink)
	adm.Handle("DELETE /admin/social-links/{id}", h.Admin.AdminDeleteSocialLink)
	adm.Handle("PATCH /admin/social-links/reorder", h.Admin.AdminReorderSocialLinks)

	// Chat (admin)
	adm.Handle("GET /admin/chats", h.Chat.AdminListChats)
	adm.Handle("GET /admin/chats/unread-count", h.Chat.AdminUnreadCount)
	adm.Handle("GET /admin/chats/{id}", h.Chat.AdminGetChat)
	adm.Handle("POST /admin/chats/{id}/messages", h.Chat.AdminSendMessage)
	adm.Handle("DELETE /admin/chats/{id}", h.Chat.AdminDeleteChat)
	adm.Handle("POST /admin/chats/{id}/ban", h.Chat.AdminBanUser)
	adm.Handle("POST /admin/chats/{id}/unban", h.Chat.AdminUnbanUser)
}
