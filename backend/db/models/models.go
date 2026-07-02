package models

import "time"

type AppVersion struct {
	ID           string    `json:"id"`
	AppID        string    `json:"app_id"`
	Version      string    `json:"version"`
	DownloadURL  string    `json:"download_url"`
	FilePath     string    `json:"file_path,omitempty"`
	ReleaseNotes string    `json:"release_notes"`
	PublishedAt  time.Time `json:"published_at"`
}

type ProjectImage struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	URL       string    `json:"url"`
	FilePath  string    `json:"file_path,omitempty"`
	AltText   string    `json:"alt_text"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// App is the commerce attachment for a Project. One-to-one with projects via
// project_id. Carries pricing, bundle ID, purchase mode, tax category, and
// (via translations) system_requirements. Display text (title/tagline/
// description/image) lives on the parent Project.
type App struct {
	ID                   string `json:"id"`
	ProjectID            string `json:"project_id"`
	BundleID             string `json:"bundle_id"`
	PriceCents           int    `json:"price_cents"`
	PurchaseMode         string `json:"purchase_mode"`
	TaxCategory          string `json:"tax_category"`
	DiscountedPriceCents *int   `json:"discounted_price_cents,omitempty"`
	// Populated by translation overlay (entity_type='app', field='system_requirements').
	SystemRequirements string     `json:"system_requirements,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
}

type Order struct {
	ID                 string    `json:"id"`
	PaymentSession     string    `json:"payment_session"`
	Email              string    `json:"email"`
	AppID              string    `json:"app_id"`
	PricePaidCents     int       `json:"price_paid_cents"`
	DiscountCodeID     *string   `json:"discount_code_id,omitempty"`
	AutoDiscountID     *string   `json:"auto_discount_id,omitempty"`
	OriginalPriceCents *int      `json:"original_price_cents,omitempty"`
	DiscountLabel      *string   `json:"discount_label,omitempty"`
	DiscountType       *string   `json:"discount_type,omitempty"`
	DiscountValue      *int      `json:"discount_value,omitempty"`
	Refunded           bool      `json:"refunded"`
	ConsentGivenAt     *string   `json:"consent_given_at,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type License struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	OrderID        string    `json:"order_id"`
	AppID          string    `json:"app_id"`
	AppBundleID    string    `json:"app_bundle_id"`
	AppName        string    `json:"app_name"`
	Revoked        bool      `json:"revoked"`
	MaxActivations *int      `json:"max_activations,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Activation struct {
	ID          string     `json:"id"`
	LicenseID   string     `json:"license_id"`
	MachineHash string     `json:"machine_hash"`
	DeviceLabel *string    `json:"device_label"`
	KeyID       *string    `json:"key_id,omitempty"`
	ActivatedAt time.Time  `json:"activated_at"`
	LastSeenAt  *time.Time `json:"last_seen_at"`
}

type AutoDiscount struct {
	ID            string     `json:"id"`
	Label         string     `json:"label"`
	DiscountType  string     `json:"discount_type"`
	DiscountValue int        `json:"discount_value"`
	AppID         *string    `json:"app_id"`
	ValidFrom     *time.Time `json:"valid_from"`
	ExpiresAt     *time.Time `json:"expires_at"`
	Active        bool       `json:"active"`
	CreatedAt     time.Time  `json:"created_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	// Stats derived from joined orders (populated by ListAutoDiscounts only).
	OrderCount   int `json:"order_count"`
	RevenueCents int `json:"revenue_cents"`
}

type ChatConversation struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	DisplayName string     `json:"display_name"`
	Email       *string    `json:"email,omitempty"`
	NotifiedAt  *time.Time `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ChatMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Sender         string    `json:"sender"`
	Body           string    `json:"body"`
	ReadAt         *string   `json:"read_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type ChatBan struct {
	UserID    string    `json:"user_id"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type SigningKey struct {
	ID                  string    `json:"id"`
	KeyID               string    `json:"key_id"`
	EncryptedPrivateKey string    `json:"-"`
	PublicKeyB64        string    `json:"public_key_b64"`
	Active              bool      `json:"active"`
	CreatedAt           time.Time `json:"created_at"`
}

type Project struct {
	ID            string     `json:"id"`
	Slug          string     `json:"slug"`
	ExternalURL   *string    `json:"external_url,omitempty"`
	ImageURL      string     `json:"image_url"`
	Position      int        `json:"position"`
	HasDetailPage bool       `json:"has_detail_page"`
	CreatedAt     time.Time  `json:"created_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	// Populated by translation overlay (not in DB)
	Title       string `json:"title"`
	Tagline     string `json:"tagline"`
	Description string `json:"description"`
	// Populated by JOIN on apps table (nil for non-commerce projects).
	Commerce *App `json:"commerce,omitempty"`
	// Populated for detail-page projects only.
	Images   []ProjectImage `json:"images,omitempty"`
	Versions []AppVersion   `json:"versions,omitempty"`
}

type SocialLink struct {
	ID        string    `json:"id"`
	Platform  string    `json:"platform"`
	URL       string    `json:"url"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

type DiscountCode struct {
	ID            string     `json:"id"`
	Code          string     `json:"code"`
	Label         string     `json:"label"`
	DiscountType  string     `json:"discount_type"`
	DiscountValue int        `json:"discount_value"`
	AppID         *string    `json:"app_id"`
	MaxUses       *int       `json:"max_uses"`
	Uses          int        `json:"uses"`
	ExpiresAt     *time.Time `json:"expires_at"`
	Active        bool       `json:"active"`
	Stackable     bool       `json:"stackable"`
	CreatedAt     time.Time  `json:"created_at"`
	DeletedAt     *time.Time `json:"deleted_at"`
	// Stats derived from joined orders (populated by ListDiscountCodes only)
	OrderCount   int `json:"order_count"`
	RevenueCents int `json:"revenue_cents"`
}
