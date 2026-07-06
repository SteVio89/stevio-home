package queries_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/db/queries"
)

// seedProjectAndApp creates a project + commerce attachment in one transaction
// and returns the resulting (projectID, appID).
func seedProjectAndApp(t *testing.T, db *sql.DB, slug, bundleID string) (string, string) {
	t.Helper()
	ctx := context.Background()
	var projectID, appID string
	err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		project, perr := queries.InsertProjectTx(ctx, tx, slug, "/media/icon.png", nil, 0, true)
		if perr != nil {
			return perr
		}
		projectID = project.ID
		app, aerr := queries.InsertAppTx(ctx, tx, project.ID, bundleID, 999, "always_new_license", "digital-goods")
		if aerr != nil {
			return aerr
		}
		appID = app.ID
		return nil
	})
	if err != nil {
		t.Fatalf("seedProjectAndApp: %v", err)
	}
	return projectID, appID
}

func TestAuthTokenLifecycle(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	email := "testhash123"
	token := "test-token-abc"
	expiresAt := time.Now().Add(15 * time.Minute)

	// Insert token
	if err := auth.InsertAuthToken(ctx, db, token, email, expiresAt); err != nil {
		t.Fatalf("InsertAuthToken: %v", err)
	}

	// Consume token successfully
	gotEmail, _, err := auth.ConsumeAuthTokenFull(ctx, db, token)
	if err != nil {
		t.Fatalf("ConsumeAuthTokenFull: %v", err)
	}
	if gotEmail != email {
		t.Errorf("email = %q, want %q", gotEmail, email)
	}

	// Consume again — should be ErrTokenUsed
	_, _, err = auth.ConsumeAuthTokenFull(ctx, db, token)
	if err != auth.ErrTokenUsed {
		t.Errorf("second consume: got %v, want ErrTokenUsed", err)
	}

	// Non-existent token
	_, _, err = auth.ConsumeAuthTokenFull(ctx, db, "nonexistent")
	if err != auth.ErrTokenNotFound {
		t.Errorf("nonexistent: got %v, want ErrTokenNotFound", err)
	}
}

func TestAuthTokenExpired(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	// Insert already-expired token
	if err := auth.InsertAuthToken(ctx, db, "expired-tok", "hash", time.Now().Add(-1*time.Minute)); err != nil {
		t.Fatalf("InsertAuthToken: %v", err)
	}

	_, _, err := auth.ConsumeAuthTokenFull(ctx, db, "expired-tok")
	if err != auth.ErrTokenExpired {
		t.Errorf("got %v, want ErrTokenExpired", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	sessionID := "session-123"
	email := "emailhash"
	expiresAt := time.Now().Add(24 * time.Hour)

	if err := auth.CreateSession(ctx, db, sessionID, email, expiresAt); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	session, err := auth.GetSession(ctx, db, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if session == nil {
		t.Fatal("session is nil")
	}
	if session.EmailHash != email {
		t.Errorf("email = %q, want %q", session.EmailHash, email)
	}

	// Delete session
	if err := auth.DeleteSession(ctx, db, sessionID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err = auth.GetSession(ctx, db, sessionID)
	if err != auth.ErrSessionNotFound {
		t.Errorf("GetSession after delete: got %v, want ErrSessionNotFound", err)
	}
}

func TestAppCRUD(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	projectID, appID := seedProjectAndApp(t, db, "crud-app", "com.test.crud")

	// Look up by project_id.
	got, err := queries.GetAppByProjectID(ctx, db, projectID)
	if err != nil {
		t.Fatalf("GetAppByProjectID: %v", err)
	}
	if got == nil || got.ID != appID {
		t.Fatalf("GetAppByProjectID returned %+v, want id=%q", got, appID)
	}
	if got.PriceCents != 999 {
		t.Errorf("price = %d, want 999", got.PriceCents)
	}

	// Look up by ID.
	byID, err := queries.GetAppByID(ctx, db, appID)
	if err != nil {
		t.Fatalf("GetAppByID: %v", err)
	}
	if byID == nil {
		t.Fatal("GetAppByID returned nil")
	}

	// Update price + purchase_mode.
	if err := queries.UpdateApp(ctx, db, appID, 1999, "always_new_license", "digital-goods"); err != nil {
		t.Fatalf("UpdateApp: %v", err)
	}
	updated, err := queries.GetAppByID(ctx, db, appID)
	if err != nil {
		t.Fatalf("GetAppByID after update: %v", err)
	}
	if updated.PriceCents != 1999 {
		t.Errorf("price = %d, want 1999", updated.PriceCents)
	}

	// Non-existent app.
	missing, err := queries.GetAppByID(ctx, db, "nonexistent")
	if err != nil {
		t.Fatalf("GetAppByID nonexistent: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for nonexistent app")
	}
}

func TestVersionsAndLatest(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	_, appID := seedProjectAndApp(t, db, "versions-app", "com.test.versions")

	v1, err := queries.InsertAppVersion(ctx, db, appID, "1.0.0", "https://example.com/v1", "")
	if err != nil {
		t.Fatalf("InsertAppVersion v1: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	v2, err := queries.InsertAppVersion(ctx, db, appID, "2.0.0", "", "app-id/v2.dmg")
	if err != nil {
		t.Fatalf("InsertAppVersion v2: %v", err)
	}

	versions, err := queries.ListVersionsByAppID(ctx, db, appID)
	if err != nil {
		t.Fatalf("ListVersionsByAppID: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}
	if versions[0].Version != "2.0.0" {
		t.Errorf("first version = %q, want 2.0.0", versions[0].Version)
	}

	latest, err := queries.GetLatestVersion(ctx, db, appID)
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if latest.ID != v2.ID {
		t.Errorf("latest = %q, want %q", latest.ID, v2.ID)
	}

	_ = v1
}

func TestOrderAndLicense(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	_, appID := seedProjectAndApp(t, db, "order-app", "com.test.order")

	err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		order, err := queries.InsertOrder(ctx, tx, "session-1", "emailhash", appID, 0, nil, nil, queries.OrderDiscountSnapshot{}, "")
		if err != nil {
			return err
		}
		_, err = queries.InsertLicense(ctx, tx, "license-key-123", order.ID, appID, nil)
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	license, err := queries.GetLicenseByKey(ctx, db, "license-key-123", "de")
	if err != nil {
		t.Fatalf("GetLicenseByKey: %v", err)
	}
	if license == nil {
		t.Fatal("license is nil")
	}
	if license.AppID != appID {
		t.Errorf("app_id = %q, want %q", license.AppID, appID)
	}

	licenses, err := queries.GetLicensesByEmail(ctx, db, "emailhash", "de")
	if err != nil {
		t.Fatalf("GetLicensesByEmail: %v", err)
	}
	if len(licenses) != 1 {
		t.Fatalf("got %d licenses, want 1", len(licenses))
	}
}

func TestActivationSlotEnforcement(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	_, appID := seedProjectAndApp(t, db, "slot-app", "com.test.slot")

	var licenseID string
	err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		order, err := queries.InsertOrder(ctx, tx, "session-slot", "emailhash", appID, 0, nil, nil, queries.OrderDiscountSnapshot{}, "")
		if err != nil {
			return err
		}
		lic, err := queries.InsertLicense(ctx, tx, "slot-key", order.ID, appID, nil)
		if err != nil {
			return err
		}
		licenseID = lic.ID
		return nil
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	for i := 0; i < 3; i++ {
		machineHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		machineHash = machineHash[:63] + string(rune('0'+i))

		err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
			_, err := queries.UpsertActivation(ctx, tx, licenseID, machineHash, "Device", nil)
			return err
		})
		if err != nil {
			t.Fatalf("activate device %d: %v", i, err)
		}
	}

	count, err := queries.CountActivations(ctx, db, licenseID)
	if err != nil {
		t.Fatalf("CountActivations: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	firstMachine := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0"
	err = queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		exists, err := queries.ActivationExistsForMachine(ctx, tx, licenseID, firstMachine)
		if err != nil {
			return err
		}
		if !exists {
			t.Error("machine should exist")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("re-activation check: %v", err)
	}
}

func TestDownloadTokenLifecycle(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	_, appID := seedProjectAndApp(t, db, "dl-app", "com.test.dl")

	var licenseID string
	err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		order, err := queries.InsertOrder(ctx, tx, "dl-session", "emailhash", appID, 0, nil, nil, queries.OrderDiscountSnapshot{}, "")
		if err != nil {
			return err
		}
		lic, err := queries.InsertLicense(ctx, tx, "dl-key", order.ID, appID, nil)
		if err != nil {
			return err
		}
		licenseID = lic.ID
		return nil
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	token := "download-token-abc"
	expiresAt := time.Now().Add(15 * time.Minute)

	if err := queries.InsertDownloadToken(ctx, db, token, licenseID, appID, expiresAt); err != nil {
		t.Fatalf("InsertDownloadToken: %v", err)
	}

	gotAppID, err := queries.ConsumeDownloadToken(ctx, db, token)
	if err != nil {
		t.Fatalf("ConsumeDownloadToken: %v", err)
	}
	if gotAppID != appID {
		t.Errorf("app_id = %q, want %q", gotAppID, appID)
	}

	_, err = queries.ConsumeDownloadToken(ctx, db, token)
	if err != queries.ErrDownloadTokenUsed {
		t.Errorf("got %v, want ErrDownloadTokenUsed", err)
	}

	_, err = queries.ConsumeDownloadToken(ctx, db, "nonexistent")
	if err != queries.ErrDownloadTokenNotFound {
		t.Errorf("got %v, want ErrDownloadTokenNotFound", err)
	}

	if err := queries.InsertDownloadToken(ctx, db, "expired-dl", licenseID, appID, time.Now().Add(-1*time.Minute)); err != nil {
		t.Fatalf("InsertDownloadToken expired: %v", err)
	}
	_, err = queries.ConsumeDownloadToken(ctx, db, "expired-dl")
	if err != queries.ErrDownloadTokenExpired {
		t.Errorf("got %v, want ErrDownloadTokenExpired", err)
	}
}

func TestCleanupExpiredTokensAndSessions(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	_ = auth.InsertAuthToken(ctx, db, "expired-1", "hash", time.Now().Add(-1*time.Hour))
	_ = auth.InsertAuthToken(ctx, db, "valid-1", "hash", time.Now().Add(1*time.Hour))

	_, _, _ = auth.ConsumeAuthTokenFull(ctx, db, "valid-1")

	_, err := auth.DeleteExpiredAuthTokens(ctx, db)
	if err != nil {
		t.Fatalf("DeleteExpiredAuthTokens: %v", err)
	}

	var tokenCount int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_tokens`).Scan(&tokenCount)
	if tokenCount != 0 {
		t.Errorf("auth_tokens remaining = %d, want 0", tokenCount)
	}

	_ = auth.CreateSession(ctx, db, "expired-session", "hash", time.Now().Add(-1*time.Hour))
	_ = auth.CreateSession(ctx, db, "valid-session", "hash", time.Now().Add(24*time.Hour))

	_, err = auth.DeleteExpiredSessions(ctx, db)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}

	_, err = auth.GetSession(ctx, db, "expired-session")
	if err != auth.ErrSessionNotFound {
		t.Errorf("expired session: got %v, want ErrSessionNotFound", err)
	}

	s, err := auth.GetSession(ctx, db, "valid-session")
	if err != nil {
		t.Fatalf("GetSession valid: %v", err)
	}
	if s == nil {
		t.Error("valid session should still exist")
	}
}

func TestProjectImages(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	projectID, _ := seedProjectAndApp(t, db, "gallery-app", "com.test.gallery")

	// Insert images.
	var img1ID, img2ID string
	if err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		i1, err := queries.InsertProjectImageTx(ctx, tx, projectID, "/media/s1.png", "project_images/s1.png", 0)
		if err != nil {
			return err
		}
		img1ID = i1.ID
		i2, err := queries.InsertProjectImageTx(ctx, tx, projectID, "/media/s2.png", "project_images/s2.png", 1)
		if err != nil {
			return err
		}
		img2ID = i2.ID
		return nil
	}); err != nil {
		t.Fatalf("InsertProjectImageTx: %v", err)
	}

	imgs, err := queries.ListProjectImages(ctx, db, projectID)
	if err != nil {
		t.Fatalf("ListProjectImages: %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("got %d images, want 2", len(imgs))
	}

	maxPos, err := queries.GetProjectImageMaxPosition(ctx, db, projectID)
	if err != nil {
		t.Fatalf("GetProjectImageMaxPosition: %v", err)
	}
	if maxPos != 1 {
		t.Errorf("maxPos = %d, want 1", maxPos)
	}

	// Delete only matches when project_id matches.
	filePath, err := queries.DeleteProjectImageForProject(ctx, db, img1ID, projectID)
	if err != nil {
		t.Fatalf("DeleteProjectImageForProject: %v", err)
	}
	if filePath != "project_images/s1.png" {
		t.Errorf("filePath = %q", filePath)
	}

	filePath, err = queries.DeleteProjectImageForProject(ctx, db, img2ID, "wrong-project")
	if err != nil {
		t.Fatalf("DeleteProjectImageForProject wrong project: %v", err)
	}
	if filePath != "" {
		t.Errorf("expected empty filePath for wrong project, got %q", filePath)
	}
}

func TestDeviceManagement(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	_, appID := seedProjectAndApp(t, db, "device-app", "com.test.device")
	emailHash := "test-email-hash"

	var licenseID string
	err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		order, err := queries.InsertOrder(ctx, tx, "dev-session", emailHash, appID, 0, nil, nil, queries.OrderDiscountSnapshot{}, "")
		if err != nil {
			return err
		}
		lic, err := queries.InsertLicense(ctx, tx, "dev-key", order.ID, appID, nil)
		if err != nil {
			return err
		}
		licenseID = lic.ID
		return nil
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	machineHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	var activationID string
	err = queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		act, err := queries.UpsertActivation(ctx, tx, licenseID, machineHash, "My MacBook", nil)
		if err != nil {
			return err
		}
		activationID = act.ID
		return nil
	})
	if err != nil {
		t.Fatalf("activate: %v", err)
	}

	act, err := queries.GetActivationByIDAndEmail(ctx, db, activationID, emailHash)
	if err != nil {
		t.Fatalf("GetActivationByIDAndEmail: %v", err)
	}
	if act == nil {
		t.Fatal("activation is nil")
	}

	act, err = queries.GetActivationByIDAndEmail(ctx, db, activationID, "wrong-hash")
	if err != nil {
		t.Fatalf("GetActivationByIDAndEmail wrong email: %v", err)
	}
	if act != nil {
		t.Error("expected nil for wrong email")
	}

	if err := queries.UpdateDeviceLabel(ctx, db, activationID, "My New MacBook"); err != nil {
		t.Fatalf("UpdateDeviceLabel: %v", err)
	}

	if err := queries.DeleteActivation(ctx, db, activationID); err != nil {
		t.Fatalf("DeleteActivation: %v", err)
	}

	count, err := queries.CountActivations(ctx, db, licenseID)
	if err != nil {
		t.Fatalf("CountActivations: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}
