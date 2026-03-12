//go:build integration

package helpers_test

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dccoding1118/more-line-points/cmd/scheduler/cli"
	"github.com/rogpeppe/go-internal/testscript"
	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"scheduler": func() int {
			ctx := context.Background()
			if err := cli.Execute(ctx); err != nil {
				return 1
			}
			return 0
		},
	}))
}

func TestIntegration(t *testing.T) {
	subDirs := []string{
		"../integration/patch0",
		"../integration/patch1",
		"../integration/patch2",
		"../integration/patch3",
	}

	// Find the project root directory
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to get root dir: %v", err)
	}

	for _, dir := range subDirs {
		t.Run(dir, func(t *testing.T) {
			testscript.Run(t, testscript.Params{
				Dir:                 dir,
				RequireExplicitExec: true,
				Setup: func(env *testscript.Env) error {
					// Initialize Mock API Server
					server := setupMockServer(env)
					env.Defer(server.Close)
					env.Vars = append(env.Vars,
						"MOCK_LINE_API="+server.URL,
						"MOCK_DISCORD_API="+server.URL,
						"ROOT_DIR="+rootDir,
					)
					return nil
				},
				Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
					"db-count":     cmdDBCount,
					"db-query":     cmdDBQuery,
					"db-insert":    cmdDBInsert,
					"mock-api":     cmdMockAPI,
					"mock-discord": cmdMockDiscord,
					"mock-email":   cmdMockEmail,
				},
			})
		})
	}
}

func setupMockServer(env *testscript.Env) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log Discord requests
		if strings.Contains(r.URL.Path, "/channels/") && strings.Contains(r.URL.Path, "/messages") {
			r.ParseMultipartForm(32 << 20) // Parse incoming discord messages
			content := r.FormValue("content")
			if content == "" {
				// sometimes discordgo uses json
				var m map[string]interface{}
				_ = m
				// wait, a simple body read works
			}
			bodyBytes, _ := io.ReadAll(r.Body)
			logPath := filepath.Join(env.WorkDir, ".mock_discord.log")
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if f != nil {
				f.WriteString(string(bodyBytes) + "\n")
				f.Close()
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id": "12345"}`))
			return
		}

		// Regular line api mock logic
		configPath := filepath.Join(env.WorkDir, ".mock_api_config")
		idxPath := filepath.Join(env.WorkDir, ".mock_api_idx")

		data, err := os.ReadFile(configPath)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		idxData, _ := os.ReadFile(idxPath)
		currentIdx, _ := strconv.Atoi(strings.TrimSpace(string(idxData)))

		responses := strings.Split(strings.TrimSpace(string(data)), "\n")
		if currentIdx >= len(responses) || responses[currentIdx] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		respFile := strings.TrimSpace(responses[currentIdx])
		if !filepath.IsAbs(respFile) {
			respFile = filepath.Join(env.WorkDir, respFile)
		}
		content, err := os.ReadFile(respFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if currentIdx < len(responses)-1 {
			currentIdx++
			_ = os.WriteFile(idxPath, []byte(strconv.Itoa(currentIdx)), 0o644)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(content)
	}))
	return server
}

func cmdMockAPI(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) == 0 {
		ts.Fatalf("usage: mock-api <json_file>...")
	}
	absDir := ts.MkAbs(".")
	configPath := filepath.Join(absDir, ".mock_api_config")
	ts.Check(os.WriteFile(configPath, []byte(strings.Join(args, "\n")), 0o644))

	// Reset index
	idxPath := filepath.Join(absDir, ".mock_api_idx")
	ts.Check(os.WriteFile(idxPath, []byte("0"), 0o644))
}

func cmdDBCount(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: db-count <table> <expected_count>")
	}
	table := args[0]
	expected, _ := strconv.Atoi(args[1])
	db := openDB(ts)
	defer db.Close()
	var count int
	ts.Check(db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count))
	if count != expected {
		ts.Fatalf("table %s count mismatch: got %d, want %d", table, count, expected)
	}
}

func cmdDBQuery(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) == 0 {
		ts.Fatalf("usage: db-query <sql>")
	}
	// Use RawArgs to get the original unparsed command line for SQL
	// Actually testscript doesn't provide RawArgs easily in the func.
	// Let's use strings.Join(args, " ") but be careful about how 'unknown' is passed.
	sqlStr := strings.Join(args, " ")
	db := openDB(ts)
	defer db.Close()
	ts.Logf("Executing SQL: %s", sqlStr)
	rows, err := db.Query(sqlStr)
	ts.Check(err)
	defer rows.Close()
	cols, _ := rows.Columns()
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptr := make([]interface{}, len(cols))
		for i := range vals {
			ptr[i] = &vals[i]
		}
		ts.Check(rows.Scan(ptr...))
		var line []string
		for _, v := range vals {
			line = append(line, fmt.Sprint(v))
		}
		ts.Stdout().Write([]byte(strings.Join(line, "\t") + "\n"))
	}
}

func cmdDBInsert(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: db-insert <fixture_file>")
	}
	sqlScript, err := os.ReadFile(ts.MkAbs(args[0]))
	ts.Check(err)
	db := openDB(ts)
	defer db.Close()
	_, err = db.Exec(string(sqlScript))
	ts.Check(err)
}

func openDB(ts *testscript.TestScript) *sql.DB {
	dbPath := filepath.Join(ts.MkAbs("."), "data/test.db")
	db, err := sql.Open("sqlite", dbPath)
	ts.Check(err)
	return db
}

func cmdMockDiscord(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 || args[0] != "contains" {
		ts.Fatalf("usage: mock-discord contains <string>")
	}
	expected := args[1]
	logPath := filepath.Join(ts.MkAbs("."), ".mock_discord.log")
	b, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		ts.Fatalf("failed to read mock-discord log: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, expected) {
		ts.Fatalf("expected discord message containing %q, but got:\n%s", expected, content)
	}
}

func cmdMockEmail(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 || args[0] != "contains" {
		ts.Fatalf("usage: mock-email contains <string>")
	}
	expected := args[1]
	logPath := filepath.Join(ts.MkAbs("."), ".mock_email.log")
	b, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		ts.Fatalf("failed to read mock-email log: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, expected) {
		ts.Fatalf("expected email containing %q, but got:\n%s", expected, content)
	}
}
