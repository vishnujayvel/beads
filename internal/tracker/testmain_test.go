//go:build cgo

package tracker

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/steveyegge/beads/internal/storage/dolt"
	"github.com/steveyegge/beads/internal/testutil"
)

// testServerPort is the port of the shared test Dolt server.
var testServerPort int

// testSharedDB is the name of the shared database for branch-per-test isolation.
var testSharedDB string

// trackerDoltRecoveryErr latches a failed container replacement so the rest of
// the package fails fast instead of retrying a 60s container start per test.
var trackerDoltRecoveryErr error

// testSharedConn is a raw *sql.DB for branch operations in the shared database.
var testSharedConn *sql.DB

func TestMain(m *testing.M) {
	os.Exit(testMainInner(m))
}

func testMainInner(m *testing.M) int {
	os.Setenv("BEADS_TEST_MODE", "1")
	if err := testutil.EnsureDoltContainerForTestMain(); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: %v, skipping Dolt tests\n", err)
	} else {
		defer testutil.TerminateDoltContainer()
		defer func() {
			if testSharedConn != nil {
				testSharedConn.Close()
			}
		}()

		if err := setupTrackerShared(); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
			return 1
		}
	}

	code := m.Run()

	os.Unsetenv("BEADS_DOLT_PORT")
	os.Unsetenv("BEADS_TEST_MODE")
	return code
}

// setupTrackerShared points the package globals at the current shared Dolt
// container and creates the shared database for branch-per-test isolation,
// with schema and config committed to main. It runs from TestMain and again
// whenever the container has to be replaced.
func setupTrackerShared() error {
	testServerPort = testutil.DoltContainerPortInt()
	testSharedDB = "tracker_pkg_shared"
	db, err := testutil.SetupSharedTestDB(testServerPort, testSharedDB)
	if err != nil {
		return fmt.Errorf("shared DB setup failed: %w", err)
	}
	testSharedConn = db
	if err := initTrackerSharedSchema(testServerPort); err != nil {
		return fmt.Errorf("shared schema init failed: %w", err)
	}
	return nil
}

// requireHealthyTrackerDolt makes sure the shared Dolt container still serves
// SQL before a test opens a store on it. In CI the container has been seen to
// drop every connection mid-suite ("unexpected EOF", then "invalid connection"),
// which used to fail every remaining test at 0.00s. When that happens, replace
// the container and rebuild the shared database so only the test that was in
// flight when it died is lost. Tests in this package run serially, which this
// relies on. The replacement is announced on stderr (with the dead container's
// state and logs) so the underlying crash stays visible even when tests pass.
func requireHealthyTrackerDolt(t *testing.T) {
	t.Helper()
	if trackerDoltRecoveryErr != nil {
		t.Fatalf("shared Dolt container could not be replaced earlier: %v", trackerDoltRecoveryErr)
	}
	if testutil.DoltServerReachable() {
		return
	}
	fmt.Fprintf(os.Stderr, "WARN: %s: shared Dolt container unreachable (%v); replacing it\n",
		t.Name(), testutil.DoltContainerCrashError())
	if testSharedConn != nil {
		_ = testSharedConn.Close()
		testSharedConn = nil
	}
	if err := testutil.RestartDoltContainer(); err != nil {
		trackerDoltRecoveryErr = fmt.Errorf("restarting container: %w", err)
		t.Fatalf("%v", trackerDoltRecoveryErr)
	}
	if err := setupTrackerShared(); err != nil {
		trackerDoltRecoveryErr = fmt.Errorf("rebuilding shared database: %w", err)
		t.Fatalf("%v", trackerDoltRecoveryErr)
	}
}

func initTrackerSharedSchema(port int) error {
	ctx := context.Background()
	cfg := &dolt.Config{
		Path:         "/tmp/tracker-shared-init",
		ServerHost:   "127.0.0.1",
		ServerPort:   port,
		Database:     testSharedDB,
		MaxOpenConns: 1,
	}
	store, err := dolt.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("New: %w", err)
	}
	defer store.Close()

	if err := store.SetConfig(ctx, "issue_prefix", "bd"); err != nil {
		return fmt.Errorf("SetConfig(issue_prefix): %w", err)
	}

	// Commit schema to main so branches get a clean snapshot
	db := store.DB()
	if _, err := db.ExecContext(ctx, "CALL DOLT_ADD('-A')"); err != nil {
		return fmt.Errorf("DOLT_ADD: %w", err)
	}
	if _, err := db.ExecContext(ctx, "CALL DOLT_COMMIT('--allow-empty', '-m', 'test: init shared schema')"); err != nil {
		return fmt.Errorf("DOLT_COMMIT: %w", err)
	}
	if err := testutil.MaterializeLocalTableSchemasForBranchTests(ctx, db); err != nil {
		return fmt.Errorf("materialize local table schemas: %w", err)
	}

	return nil
}
