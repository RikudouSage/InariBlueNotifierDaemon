// Package contacts provides the local, contact-aware helper service used by
// the notifier. It deliberately exposes only favourite-number lookups.
package contacts

import (
	"InariBlueNotifier/config"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/godbus/dbus/v5"
	_ "modernc.org/sqlite"
)

const (
	// ServiceName is owned by the contacts helper on the user's session bus.
	ServiceName   = "dev.rikudou.InariBlueNotifierContacts"
	objectPath    = dbus.ObjectPath("/dev/rikudou/InariBlueNotifierContacts")
	interfaceName = ServiceName

	notifierService = "inari-blue-notifier-daemon.service"
)

// DefaultDatabasePath returns Sailfish's privileged QtContacts database path.
func DefaultDatabasePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "system", "privileged", "Contacts", "qtcontacts-sqlite", "contacts.db"), nil
}

// Store provides read-only favourite-number lookups.
type Store struct {
	database *sql.DB
}

func NewStore(databasePath string) (*Store, error) {
	if databasePath == "" {
		return nil, errors.New("contacts database path is empty")
	}
	info, err := os.Stat(databasePath)
	if err != nil {
		return nil, fmt.Errorf("stat contacts database %q: %w", databasePath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("contacts database path is a directory: %q", databasePath)
	}
	file, err := os.Open(databasePath)
	if err != nil {
		return nil, fmt.Errorf("open contacts database %q for reading: %w", databasePath, err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close contacts database %q after access check: %w", databasePath, err)
	}

	database, err := sql.Open("sqlite", "file:"+databasePath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open contacts database: %w", err)
	}
	database.SetMaxOpenConns(1)

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("connect to contacts database: %w", err)
	}

	return &Store{database: database}, nil
}

func (receiver *Store) Close() error {
	return receiver.database.Close()
}

// IsFavorite reports whether a phone number belongs to a favourite contact.
func (receiver *Store) IsFavorite(ctx context.Context, number string) (bool, error) {
	normalized := normalizePhoneNumber(number)
	if normalized == "" {
		return false, nil
	}

	const query = `SELECT EXISTS (
		SELECT 1
		FROM PhoneNumbers AS phone_numbers
		JOIN Favorites AS favorites ON favorites.contactId = phone_numbers.contactId
		WHERE phone_numbers.normalizedNumber = ?
		  AND favorites.isFavorite = 1
	)`

	var favourite bool
	if err := receiver.database.QueryRowContext(ctx, query, normalized).Scan(&favourite); err != nil {
		return false, fmt.Errorf("query favourite contact: %w", err)
	}
	return favourite, nil
}

// Server exposes favourite lookups only to the live notifier service.
type Server struct {
	connection *dbus.Conn
	store      *Store
	closeOnce  sync.Once

	logger *config.Logger
}

// NewServer connects to the session bus and registers the helper service.
func NewServer(store *Store, logger *config.Logger) (*Server, error) {
	if store == nil {
		return nil, errors.New("contacts store is nil")
	}

	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to session bus: %w", err)
	}

	server := &Server{connection: connection, store: store, logger: logger}
	if err := connection.Export(server, objectPath, interfaceName); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("export contacts service: %w", err)
	}
	result, err := connection.RequestName(ServiceName, dbus.NameFlagDoNotQueue)
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("claim contacts service name: %w", err)
	}
	if result != dbus.RequestNameReplyPrimaryOwner {
		_ = connection.Close()
		return nil, fmt.Errorf("claim contacts service name: unexpected result %s", result)
	}

	return server, nil
}

func (receiver *Server) Close() error {
	var result error
	receiver.closeOnce.Do(func() {
		_, releaseErr := receiver.connection.ReleaseName(ServiceName)
		result = errors.Join(releaseErr, receiver.connection.Close())
	})
	return result
}

// IsFavorite is the D-Bus method used by the main notifier. dbus.Sender is
// supplied by the bus and cannot be forged by the client.
func (receiver *Server) IsFavorite(sender dbus.Sender, number string) (bool, *dbus.Error) {
	if err := receiver.authorize(sender); err != nil {
		receiver.logger.Warn("rejected contacts lookup", slog.String("sender", string(sender)), slog.String("error", err.Error()))
		return false, dbus.MakeFailedError(errors.New("caller is not the notifier service"))
	}

	favourite, err := receiver.store.IsFavorite(context.Background(), number)
	if err != nil {
		receiver.logger.Error("contacts lookup failed", slog.String("error", err.Error()))
		return false, dbus.MakeFailedError(errors.New("contacts lookup failed"))
	}
	return favourite, nil
}

// authorize compares bus-authenticated client credentials with the live
// MainPID recorded for the notifier's user systemd service. This remains valid
// across the helper's private PID namespace.
func (receiver *Server) authorize(sender dbus.Sender) error {
	var uid uint32
	if err := receiver.connection.BusObject().Call(
		"org.freedesktop.DBus.GetConnectionUnixUser", 0, string(sender),
	).Store(&uid); err != nil {
		return fmt.Errorf("get caller uid: %w", err)
	}
	if uid != uint32(os.Getuid()) {
		return fmt.Errorf("caller uid=%d does not match helper uid", uid)
	}

	var callerPID uint32
	if err := receiver.connection.BusObject().Call(
		"org.freedesktop.DBus.GetConnectionUnixProcessID", 0, string(sender),
	).Store(&callerPID); err != nil {
		return fmt.Errorf("get caller pid: %w", err)
	}

	manager := receiver.connection.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	var unitPath dbus.ObjectPath
	if err := manager.Call("org.freedesktop.systemd1.Manager.GetUnit", 0, notifierService).Store(&unitPath); err != nil {
		return fmt.Errorf("get notifier unit: %w", err)
	}
	mainPID, err := receiver.connection.Object("org.freedesktop.systemd1", unitPath).GetProperty(
		"org.freedesktop.systemd1.Service.MainPID",
	)
	if err != nil {
		return fmt.Errorf("get notifier MainPID: %w", err)
	}
	pid, ok := mainPID.Value().(uint32)
	if !ok {
		return fmt.Errorf("unexpected notifier MainPID type %T", mainPID.Value())
	}
	if callerPID != pid || pid == 0 {
		return fmt.Errorf("caller pid=%d does not match notifier MainPID=%d", callerPID, pid)
	}
	return nil
}

// Client calls the helper over the session bus. Authentication is performed by
// the helper from the bus-provided sender identity.
type Client struct {
	connection *dbus.Conn
}

func NewClient(connection *dbus.Conn) *Client {
	return &Client{connection: connection}
}

func (receiver *Client) IsFavorite(ctx context.Context, number string) (bool, error) {
	if receiver.connection == nil {
		return false, errors.New("session bus connection is nil")
	}
	var favourite bool
	if err := receiver.connection.Object(ServiceName, objectPath).CallWithContext(
		ctx, interfaceName+".IsFavorite", 0, number,
	).Store(&favourite); err != nil {
		return false, fmt.Errorf("call contacts helper: %w", err)
	}
	return favourite, nil
}

func normalizePhoneNumber(number string) string {
	// This mirrors QtContactsSqliteExtensions::minimizePhoneNumber() from
	// Sailfish's Qt 5.6 qtcontacts-sqlite backend. PhoneNumbers.normalizedNumber
	// contains its result, so lookup must use the same representation.
	if strings.HasPrefix(number, "sips:") || strings.HasPrefix(number, ":") {
		if colon := strings.IndexRune(number, ':'); colon >= 0 {
			if at := strings.IndexRune(number[colon+1:], '@'); at >= 0 {
				number = number[colon+1 : colon+1+at]
			}
		}
	}

	const separators = " .-()[]"
	const dtmf = "pPwWxX#*"

	result := make([]rune, 0, len(number))
	initialCharacter := false
	hasNumericComponent := false
	firstDTMF := -1

	for _, character := range number {
		switch {
		case unicode.IsDigit(character):
			result = append(result, rune('0'+decimalDigitValue(character)))
			hasNumericComponent = true
			initialCharacter = true

		case character == '+':
			if !initialCharacter {
				result = append(result, character)
				initialCharacter = true
			} else if firstDTMF != -1 {
				result = append(result, character)
			}

		case strings.ContainsRune(separators, character):
			// Qt removes formatting punctuation when minimizing a number.

		case strings.ContainsRune(dtmf, character):
			if unicode.IsLetter(character) && !hasNumericComponent {
				// Qt silently skips alphabetic DTMF characters before the number
				// when validation is disabled (as it is for the stored key).
				continue
			}
			if firstDTMF == -1 {
				firstDTMF = len(result)
			}
			if character == 'x' || character == 'X' {
				character = 'p'
			}
			result = append(result, character)
		}
	}

	start := len(result) - 1
	if firstDTMF != -1 {
		start = firstDTMF - 1
	}
	characters := 0
	for index := start; index > 0; index-- {
		if (result[index] >= '0' && result[index] <= '9') || result[index] == '+' {
			characters++
			if characters == 8 {
				result = result[index:]
				break
			}
		}
	}

	return strings.TrimSpace(string(result))
}

func decimalDigitValue(character rune) int {
	if character >= '0' && character <= '9' {
		return int(character - '0')
	}

	for _, digitRange := range unicode.Digit.R16 {
		if character >= rune(digitRange.Lo) && character <= rune(digitRange.Hi) && (character-rune(digitRange.Lo))%rune(digitRange.Stride) == 0 {
			return int((character - rune(digitRange.Lo)) / rune(digitRange.Stride) % 10)
		}
	}
	for _, digitRange := range unicode.Digit.R32 {
		if character >= rune(digitRange.Lo) && character <= rune(digitRange.Hi) && (character-rune(digitRange.Lo))%rune(digitRange.Stride) == 0 {
			return int((character - rune(digitRange.Lo)) / rune(digitRange.Stride) % 10)
		}
	}

	return 0
}
