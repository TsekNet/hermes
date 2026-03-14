package cmd

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/google/deck"
	"golang.org/x/sys/windows"
)

var (
	modwtsapi32 = windows.NewLazySystemDLL("wtsapi32.dll")
	moduserenv  = windows.NewLazySystemDLL("userenv.dll")

	procWTSEnumerateSessions = modwtsapi32.NewProc("WTSEnumerateSessionsW")
	procWTSFreeMemory        = modwtsapi32.NewProc("WTSFreeMemory")
	procWTSQueryUserToken    = modwtsapi32.NewProc("WTSQueryUserToken")
	procCreateEnvBlock       = moduserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvBlock      = moduserenv.NewProc("DestroyEnvironmentBlock")
)

const (
	wtsActive            = 0
	tokenPrimary         = 1
	securityImperson     = 2
	createUnicodeEnv     = 0x00000400
	createNoWindow       = 0x08000000
	createBreakawayJob   = 0x01000000
)

type wtsSessionInfo struct {
	SessionID      uint32
	WinStationName *uint16
	State          uint32
}

// isPrivileged reports whether the current process is running as SYSTEM.
// MSI custom actions run as SYSTEM; manual elevated Admin installs do not
// trigger session launch (by design: the user's own session already has
// the installer running interactively).
func isPrivileged() bool {
	t, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer t.Close()

	user, err := t.GetTokenUser()
	if err != nil {
		return false
	}

	return user.User.Sid.IsWellKnown(windows.WinLocalSystemSid)
}

func launchInUserSessions(exe string, args []string) {
	sessions, err := enumerateActiveSessions()
	if err != nil {
		deck.Warningf("session launch: enumerate sessions: %v", err)
		return
	}

	cmdLine := buildCommandLine(exe, args)

	launched := make(map[string]bool)
	for _, sid := range sessions {
		token, username, err := acquireSessionToken(sid)
		if err != nil {
			deck.Warningf("session launch: session %d: %v", sid, err)
			continue
		}
		if username != "" && launched[username] {
			token.Close()
			deck.Infof("session launch: session %d: skipped (already launched for %s)", sid, username)
			continue
		}
		if err := launchWithToken(token, exe, cmdLine); err != nil {
			token.Close()
			deck.Warningf("session launch: session %d: %v", sid, err)
			continue
		}
		token.Close()
		if username != "" {
			launched[username] = true
		}
		deck.Infof("session launch: started hermes serve in session %d (%s)", sid, username)
	}
}

func enumerateActiveSessions() ([]uint32, error) {
	var (
		pSessionInfo uintptr
		count        uint32
	)

	r1, _, err := procWTSEnumerateSessions.Call(
		0, // WTS_CURRENT_SERVER_HANDLE
		0,
		1,
		uintptr(unsafe.Pointer(&pSessionInfo)),
		uintptr(unsafe.Pointer(&count)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("WTSEnumerateSessionsW: %w", err)
	}
	defer procWTSFreeMemory.Call(pSessionInfo)

	entrySize := unsafe.Sizeof(wtsSessionInfo{})
	var active []uint32
	for i := uint32(0); i < count; i++ {
		entry := (*wtsSessionInfo)(unsafe.Pointer(pSessionInfo + uintptr(i)*entrySize))
		if entry.State == wtsActive && entry.SessionID != 0 {
			active = append(active, entry.SessionID)
		}
	}
	return active, nil
}

const tokenMinAccess = windows.TOKEN_QUERY | windows.TOKEN_DUPLICATE | windows.TOKEN_ASSIGN_PRIMARY

// acquireSessionToken gets a primary token for the session and resolves the username.
// Caller must call token.Close() when done.
func acquireSessionToken(sessionID uint32) (windows.Token, string, error) {
	var impToken windows.Handle
	r1, _, err := procWTSQueryUserToken.Call(
		uintptr(sessionID),
		uintptr(unsafe.Pointer(&impToken)),
	)
	if r1 == 0 {
		return 0, "", fmt.Errorf("WTSQueryUserToken: %w", err)
	}
	defer windows.CloseHandle(impToken)

	var token windows.Token
	if err := windows.DuplicateTokenEx(
		windows.Token(impToken), tokenMinAccess, nil,
		securityImperson, tokenPrimary, &token,
	); err != nil {
		return 0, "", fmt.Errorf("DuplicateTokenEx: %w", err)
	}

	tu, err := token.GetTokenUser()
	if err != nil {
		token.Close()
		return 0, "", fmt.Errorf("GetTokenUser: %w", err)
	}
	username, _, _, _ := tu.User.Sid.LookupAccount("")
	return token, username, nil
}

func launchWithToken(userToken windows.Token, exe, cmdLine string) error {
	var envBlock uintptr
	r1, _, err := procCreateEnvBlock.Call(
		uintptr(unsafe.Pointer(&envBlock)),
		uintptr(userToken),
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("CreateEnvironmentBlock: %w", err)
	}
	defer procDestroyEnvBlock.Call(envBlock)

	startupInfo := new(windows.StartupInfo)
	startupInfo.Cb = uint32(unsafe.Sizeof(*startupInfo))
	startupInfo.Desktop = windows.StringToUTF16Ptr(`winsta0\default`)
	startupInfo.ShowWindow = windows.SW_HIDE
	startupInfo.Flags = windows.STARTF_USESHOWWINDOW

	var procInfo windows.ProcessInformation

	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return fmt.Errorf("UTF16 command line: %w", err)
	}
	exePtr, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("UTF16 exe path: %w", err)
	}

	if err := windows.CreateProcessAsUser(
		userToken, exePtr, cmdLinePtr, nil, nil, false,
		createUnicodeEnv|createNoWindow|createBreakawayJob,
		(*uint16)(unsafe.Pointer(envBlock)), nil, startupInfo, &procInfo,
	); err != nil {
		return fmt.Errorf("CreateProcessAsUser: %w", err)
	}

	windows.CloseHandle(windows.Handle(procInfo.Thread))
	windows.CloseHandle(windows.Handle(procInfo.Process))
	return nil
}

// buildCommandLine constructs a Windows command line string with proper quoting
// per CommandLineToArgvW rules: backslashes before a closing quote must be doubled.
func buildCommandLine(exe string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, quoteArg(exe))
	for _, a := range args {
		parts = append(parts, quoteArg(a))
	}
	return strings.Join(parts, " ")
}

func quoteArg(s string) string {
	if s == "" || strings.ContainsAny(s, ` "\`) {
		var b strings.Builder
		b.WriteByte('"')
		backslashes := 0
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '\\':
				backslashes++
			case '"':
				// Double backslashes before a quote, then escape the quote.
				b.WriteString(strings.Repeat(`\`, backslashes*2+1))
				backslashes = 0
				b.WriteByte('"')
			default:
				// Flush accumulated backslashes as-is (not before a quote).
				if backslashes > 0 {
					b.WriteString(strings.Repeat(`\`, backslashes))
					backslashes = 0
				}
				b.WriteByte(s[i])
			}
		}
		// Double trailing backslashes (they precede the closing quote).
		b.WriteString(strings.Repeat(`\`, backslashes*2))
		b.WriteByte('"')
		return b.String()
	}
	return s
}
