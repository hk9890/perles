package infrastructure

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	appbeads "github.com/hk9890/perles/internal/beads/application"
	domain "github.com/hk9890/perles/internal/beads/domain"
	"github.com/hk9890/perles/internal/log"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

var _ appbeads.ReadClient = (*DoltClient)(nil)

type DoltClient struct {
	db       *sql.DB
	details  ConnectionDetails
	beadsDir string
}

type ConnectionDetails struct {
	Host     string
	Port     int
	Database string
}

type beadsMetadata struct {
	Backend      string `json:"backend"`
	DoltMode     string `json:"dolt_mode"`
	DoltDatabase string `json:"dolt_database"`
}

type doltServerConfig struct {
	Listener struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"listener"`
}

func NewDoltClient(beadsDir string) (*DoltClient, error) {
	details, err := ResolveConnectionDetails(beadsDir)
	if err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("root@tcp(%s:%d)/%s?parseTime=true", details.Host, details.Port, details.Database)
	log.Debug(log.CatDB, "Opening Dolt database", "host", details.Host, "port", details.Port, "database", details.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening dolt mysql connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging dolt mysql connection: %w", err)
	}

	return &DoltClient{db: db, details: details, beadsDir: beadsDir}, nil
}

func (c *DoltClient) Close() error {
	return c.db.Close()
}

func (c *DoltClient) DB() *sql.DB {
	return c.db
}

func (c *DoltClient) Version() (string, error) {
	var version string
	err := c.db.QueryRow("SELECT value FROM metadata WHERE `key` = ?", "bd_version").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("reading bd_version from metadata: %w", err)
	}
	return version, nil
}

func (c *DoltClient) GetComments(issueID string) ([]domain.Comment, error) {
	query := `
		SELECT id, author, text, created_at
		FROM comments
		WHERE issue_id = ?
		ORDER BY created_at ASC
	`

	rows, err := c.db.Query(query, issueID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var comments []domain.Comment
	for rows.Next() {
		var comment domain.Comment
		if err := rows.Scan(&comment.ID, &comment.Author, &comment.Text, &comment.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	return comments, rows.Err()
}

func ResolveConnectionDetails(beadsDir string) (ConnectionDetails, error) {
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath) //nolint:gosec // metadataPath is a fixed file under the resolved .beads directory
	if err != nil {
		return ConnectionDetails{}, fmt.Errorf("reading beads metadata %q: %w", metadataPath, err)
	}

	var metadata beadsMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return ConnectionDetails{}, fmt.Errorf("parsing beads metadata %q: %w", metadataPath, err)
	}

	if metadata.Backend != "dolt" {
		return ConnectionDetails{}, fmt.Errorf("unsupported beads backend %q; expected dolt", metadata.Backend)
	}
	if metadata.DoltMode != "server" {
		return ConnectionDetails{}, fmt.Errorf("unsupported dolt mode %q; expected server", metadata.DoltMode)
	}
	if strings.TrimSpace(metadata.DoltDatabase) == "" {
		return ConnectionDetails{}, errors.New("missing dolt_database in beads metadata")
	}

	host, cfgPort, err := readDoltConfig(filepath.Join(beadsDir, "dolt", "config.yaml"))
	if err != nil {
		return ConnectionDetails{}, err
	}

	port, err := readDoltPort(filepath.Join(beadsDir, "dolt-server.port"), cfgPort)
	if err != nil {
		return ConnectionDetails{}, err
	}

	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}

	return ConnectionDetails{
		Host:     host,
		Port:     port,
		Database: metadata.DoltDatabase,
	}, nil
}

func readDoltConfig(path string) (host string, port int, err error) {
	content, err := os.ReadFile(path) //nolint:gosec // path points to the tracked Dolt config under the resolved .beads directory
	if err != nil {
		return "", 0, fmt.Errorf("reading dolt config %q: %w", path, err)
	}

	var cfg doltServerConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return "", 0, fmt.Errorf("parsing dolt config %q: %w", path, err)
	}

	if cfg.Listener.Port == 0 {
		return "", 0, fmt.Errorf("missing listener.port in dolt config %q", path)
	}

	return cfg.Listener.Host, cfg.Listener.Port, nil
}

func readDoltPort(path string, fallback int) (int, error) {
	content, err := os.ReadFile(path) //nolint:gosec // path points to the tracked Dolt port file under the resolved .beads directory
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return 0, fmt.Errorf("reading dolt server port file %q: %w", path, err)
	}

	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return fallback, nil
	}

	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parsing dolt server port file %q: %w", path, err)
	}

	if port <= 0 {
		return 0, fmt.Errorf("invalid dolt server port %d in %q", port, path)
	}

	return port, nil
}
