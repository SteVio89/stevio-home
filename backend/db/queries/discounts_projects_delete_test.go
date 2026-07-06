package queries_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/testutil"
)

// entity_type values mirror common.EntityTypeProject / ...ProjectImage (kept as
// literals here to avoid importing the handlers layer from a queries test).
const (
	entityProject      = "project"
	entityProjectImage = "project_image"
)

// TestInsertDiscountCode_NilMaxUses guards the COALESCE fix: a blank max_uses
// (nil *int → NULL) must land as 0 (= unlimited), not violate NOT NULL.
func TestInsertDiscountCode_NilMaxUses(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	code, err := queries.InsertDiscountCode(ctx, db, queries.InsertDiscountCodeParams{
		Code:          "BLANKMAX",
		DiscountType:  "percent",
		DiscountValue: 10,
		MaxUses:       nil, // the crash trigger before the fix
	})
	if err != nil {
		t.Fatalf("InsertDiscountCode with nil MaxUses: %v", err)
	}
	if code.MaxUses == nil || *code.MaxUses != 0 {
		t.Errorf("MaxUses = %v, want 0 (unlimited)", code.MaxUses)
	}

	// Update with nil must also coalesce to 0 rather than error.
	updated, err := queries.UpdateDiscountCode(ctx, db, code.ID, queries.UpdateDiscountCodeParams{
		DiscountType:  "percent",
		DiscountValue: 15,
		MaxUses:       nil,
		Active:        true,
	})
	if err != nil {
		t.Fatalf("UpdateDiscountCode with nil MaxUses: %v", err)
	}
	if updated.MaxUses == nil || *updated.MaxUses != 0 {
		t.Errorf("after update MaxUses = %v, want 0", updated.MaxUses)
	}
}

// TestHardDeleteDiscountCode checks the archived-only guard: hard-delete works
// only after archiving; a non-archived code is refused and left intact.
func TestHardDeleteDiscountCode(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	code, err := queries.InsertDiscountCode(ctx, db, queries.InsertDiscountCodeParams{
		Code: "PURGE", DiscountType: "percent", DiscountValue: 10,
	})
	if err != nil {
		t.Fatalf("InsertDiscountCode: %v", err)
	}

	// Not archived yet → refuse, row stays.
	if err := queries.HardDeleteDiscountCode(ctx, db, code.ID); !errors.Is(err, queries.ErrDiscountNotFound) {
		t.Errorf("hard-delete non-archived: got %v, want ErrDiscountNotFound", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM discount_codes WHERE id = $1`, code.ID); got != 1 {
		t.Fatalf("after refused hard-delete, rows = %d, want 1", got)
	}

	// Archive, then hard-delete → gone.
	if err := queries.SoftDeleteDiscountCode(ctx, db, code.ID); err != nil {
		t.Fatalf("SoftDeleteDiscountCode: %v", err)
	}
	if err := queries.HardDeleteDiscountCode(ctx, db, code.ID); err != nil {
		t.Fatalf("HardDeleteDiscountCode after archive: %v", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM discount_codes WHERE id = $1`, code.ID); got != 0 {
		t.Errorf("after hard-delete, rows = %d, want 0", got)
	}
}

// TestHardDeleteProject_NoOrders verifies a full purge (project, app, images,
// translations) when there is no order history.
func TestHardDeleteProject_NoOrders(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()
	testutil.SeedLocale(t, db, "de", "Deutsch", true)

	projectID, _ := seedProjectAndApp(t, db, "purge-me", "com.test.purge")
	testutil.SeedEntityTranslation(t, db, entityProject, projectID, "de", "title", "Titel")

	var imgID string
	if err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		img, err := queries.InsertProjectImageTx(ctx, tx, projectID, "/media/x.png", "project_images/x.png", 0)
		if err != nil {
			return err
		}
		imgID = img.ID
		return nil
	}); err != nil {
		t.Fatalf("seed image: %v", err)
	}
	testutil.SeedEntityTranslation(t, db, entityProjectImage, imgID, "de", "alt_text", "Alt")

	files, err := queries.HardDeleteProject(ctx, db, projectID, entityProject, entityProjectImage)
	if err != nil {
		t.Fatalf("HardDeleteProject: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected image file paths returned for unlinking")
	}

	if got := countRows(t, db, `SELECT COUNT(*) FROM projects WHERE id = $1`, projectID); got != 0 {
		t.Errorf("projects rows = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM apps WHERE project_id = $1`, projectID); got != 0 {
		t.Errorf("apps rows = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM entity_translations WHERE entity_id IN ($1, $2)`, projectID, imgID); got != 0 {
		t.Errorf("entity_translations rows = %d, want 0", got)
	}
}

// TestHardDeleteProject_WithOrders verifies the retention guard: a project whose
// app has an order is refused and everything is left intact.
func TestHardDeleteProject_WithOrders(t *testing.T) {
	db := setupAppDB(t)
	ctx := context.Background()

	projectID, appID := seedProjectAndApp(t, db, "has-sales", "com.test.sales")
	if err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		_, err := queries.InsertOrder(ctx, tx, "sess-sales", "emailhash", appID, 0, nil, nil, queries.OrderDiscountSnapshot{}, "")
		return err
	}); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	_, err := queries.HardDeleteProject(ctx, db, projectID, entityProject, entityProjectImage)
	if !errors.Is(err, queries.ErrProjectHasOrders) {
		t.Fatalf("got %v, want ErrProjectHasOrders", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM projects WHERE id = $1`, projectID); got != 1 {
		t.Errorf("projects rows = %d, want 1 (must survive)", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM apps WHERE project_id = $1`, projectID); got != 1 {
		t.Errorf("apps rows = %d, want 1 (must survive)", got)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("countRows: %v", err)
	}
	return n
}
