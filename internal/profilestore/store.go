// Пакет profilestore читает и пишет YAML-профили в каталоге ~/.config/git-herder.
package profilestore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
)

const minCreateYAML = "root: {}\n"

// ErrConfig — ошибка настроек профиля (exit 1).
var ErrConfig = errors.New("profile configuration error")

// Store работает с каталогом профилей на диске.
type Store struct {
	dir string
}

// New создаёт Store с каноническим каталогом пользователя.
func New() (*Store, error) {
	dir, err := DefaultDir()
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// NewAt создаёт Store с явным каталогом (тесты).
func NewAt(dir string) *Store {
	return &Store{dir: dir}
}

// Dir возвращает каталог профилей.
func (s *Store) Dir() string { return s.dir }

// DefaultDir — ~/.config/git-herder.
func DefaultDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("GITHERD_PROFILE_DIR")); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "git-herder"), nil
}

// ValidateProfileName проверяет имя профиля по спеке cli/profile.
func ValidateProfileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: profile name is empty", ErrConfig)
	}
	if name == "default" {
		return fmt.Errorf("%w: profile name %q is reserved", ErrConfig, name)
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("%w: invalid profile name %q", ErrConfig, name)
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: invalid profile name %q", ErrConfig, name)
	}
	return nil
}

// ListYAMLNames возвращает имена *.yaml (не .yml), отсортированные лексикографически.
func (s *Store) ListYAMLNames() (names []string, err error) {
	leave := logging.Enter("dir", s.dir)
	defer func() { leave("names", names, "err", err) }()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".yaml") {
			names = append(names, strings.TrimSuffix(n, ".yaml"))
		}
	}
	sort.Strings(names)
	return names, nil
}

// ReadDefaultName читает файл default; пустая строка — файла нет или пусто.
func (s *Store) ReadDefaultName() (string, error) {
	path := filepath.Join(s.dir, "default")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// BrokenDefault возвращает имя из default, если yaml отсутствует.
func (s *Store) BrokenDefault() (broken bool, name string, err error) {
	name, err = s.ReadDefaultName()
	if err != nil || name == "" {
		return false, "", err
	}
	ok, err := s.Exists(name)
	if err != nil {
		return false, "", err
	}
	return !ok, name, nil
}

// Exists проверяет наличие <name>.yaml.
func (s *Store) Exists(name string) (bool, error) {
	path := s.profilePath(name)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// ReadYAMLBytes читает сырой файл профиля.
func (s *Store) ReadYAMLBytes(name string) ([]byte, error) {
	path := s.profilePath(name)
	return os.ReadFile(path)
}

// ParseYAML проверяет синтаксис YAML.
func (s *Store) ParseYAML(data []byte) error {
	var doc any
	return yaml.Unmarshal(data, &doc)
}

// ReadYAMLMap читает профиль в map для маскировки и резолва.
func (s *Store) ReadYAMLMap(name string) (doc map[string]any, err error) {
	leave := logging.Enter("name", name, "path", s.profilePath(name))
	defer func() { leave("err", err) }()
	b, err := s.ReadYAMLBytes(name)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	return doc, nil
}

// WriteDefault записывает файл default.
func (s *Store) WriteDefault(name string) (err error) {
	leave := logging.Enter("name", name, "path", filepath.Join(s.dir, "default"))
	defer func() { leave("err", err) }()
	if err := s.EnsureDir(); err != nil {
		return err
	}
	path := filepath.Join(s.dir, "default")
	return os.WriteFile(path, []byte(name+"\n"), 0o600)
}

// CreateEmpty создаёт профиль root: {}.
func (s *Store) CreateEmpty(name string) (err error) {
	leave := logging.Enter("name", name, "path", s.profilePath(name))
	defer func() { leave("err", err) }()
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	if err := s.EnsureDir(); err != nil {
		return err
	}
	exists, err := s.Exists(name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%w: profile %q already exists", ErrConfig, name)
	}
	return os.WriteFile(s.profilePath(name), []byte(minCreateYAML), 0o600)
}

// CopyFrom копирует существующий профиль.
func (s *Store) CopyFrom(name, from string) (err error) {
	leave := logging.Enter("name", name, "from", from, "path", s.profilePath(name))
	defer func() { leave("err", err) }()
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	ok, err := s.Exists(from)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: source profile %q not found", ErrConfig, from)
	}
	exists, err := s.Exists(name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%w: profile %q already exists", ErrConfig, name)
	}
	if err := s.EnsureDir(); err != nil {
		return err
	}
	b, err := s.ReadYAMLBytes(from)
	if err != nil {
		return err
	}
	return os.WriteFile(s.profilePath(name), b, 0o600)
}

// Delete удаляет <name>.yaml.
func (s *Store) Delete(name string) (err error) {
	leave := logging.Enter("name", name, "path", s.profilePath(name))
	defer func() { leave("err", err) }()
	return os.Remove(s.profilePath(name))
}

// EnsureDir создаёт каталог профилей при отсутствии.
func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.dir, 0o700)
}

// ProfilePathAbs возвращает абсолютный путь к файлу профиля.
func (s *Store) ProfilePathAbs(name string) (string, error) {
	if err := ValidateProfileName(name); err != nil {
		return "", err
	}
	return filepath.Abs(s.profilePath(name))
}

func (s *Store) profilePath(name string) string {
	return filepath.Join(s.dir, name+".yaml")
}

// LoadForResolve загружает профиль по имени или default; валидирует семантику root SCM-ключей.
func (s *Store) LoadForResolve(explicitProfile string) (profileName string, doc map[string]any, err error) {
	leave := logging.Enter("explicit", explicitProfile, "dir", s.dir)
	defer func() { leave("profile", profileName, "err", err) }()
	name := strings.TrimSpace(explicitProfile)
	if name == "" {
		name, err = s.ReadDefaultName()
		if err != nil {
			return "", nil, err
		}
	}
	if name == "" {
		return "", nil, nil
	}
	ok, err := s.Exists(name)
	if err != nil {
		return "", nil, err
	}
	if !ok {
		return "", nil, fmt.Errorf("%w: profile %q not found", ErrConfig, name)
	}
	doc, err = s.ReadYAMLMap(name)
	if err != nil {
		return "", nil, err
	}
	if err := validateRootSCMKeys(doc); err != nil {
		return "", nil, err
	}
	return name, doc, nil
}

func validateRootSCMKeys(doc map[string]any) error {
	for key, val := range doc {
		block, ok := val.(map[string]any)
		if !ok {
			continue
		}
		for k := range block {
			if scmKeyOnlyInRoot(k) && key != "root" {
				return fmt.Errorf("%w: upstream, host, token, and git-https-token only allowed in root", ErrConfig)
			}
		}
		for subK, subV := range block {
			if subMap, ok := subV.(map[string]any); ok {
				if err := scanNestedCommands(subMap, key+"."+subK); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func scmKeyOnlyInRoot(k string) bool {
	switch k {
	case "upstream", "host", "token", "git-https-token":
		return true
	default:
		return false
	}
}

func scanNestedCommands(block map[string]any, path string) error {
	for k, v := range block {
		if scmKeyOnlyInRoot(k) {
			return fmt.Errorf("%w: %s must not contain upstream, host, token, or git-https-token", ErrConfig, path)
		}
		if sub, ok := v.(map[string]any); ok {
			if err := scanNestedCommands(sub, path+"."+k); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyFile имитирует копирование через io для тестов coverage.
func CopyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
