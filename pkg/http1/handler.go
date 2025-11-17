package http1

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/perbu/GTest/pkg/vtc"
)

// Handler processes HTTP command specifications
type Handler struct {
	HTTP    *HTTP
	Context interface{} // ExecContext for global commands (optional)
}

// NewHandler creates a new HTTP command handler
func NewHandler(h *HTTP) *Handler {
	return &Handler{HTTP: h}
}

// SetContext sets the execution context for global command support
func (h *Handler) SetContext(ctx interface{}) {
	h.Context = ctx
}

// ProcessSpec processes an HTTP command specification string
// This is the main entry point for executing HTTP commands from VTC specs
func (h *Handler) ProcessSpec(spec string) error {
	h.HTTP.Logger.Debug("ProcessSpec called with spec length: %d", len(spec))

	// Parse the spec into lines
	lines := strings.Split(spec, "\n")
	h.HTTP.Logger.Debug("ProcessSpec parsed %d lines", len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		h.HTTP.Logger.Debug("Processing line %d: %s", i+1, line)

		// Parse the command line
		err := h.ProcessCommand(line)
		if err != nil {
			h.HTTP.Logger.Debug("Command failed on line %d: %v", i+1, err)
			return fmt.Errorf("command '%s' failed: %w", line, err)
		}

		h.HTTP.Logger.Debug("Line %d completed successfully", i+1)
	}

	h.HTTP.Logger.Debug("ProcessSpec completed successfully")
	return nil
}

// ProcessCommand processes a single HTTP command
func (h *Handler) ProcessCommand(cmdLine string) error {
	// Tokenize the command line
	tokens := tokenizeCommand(cmdLine)
	if len(tokens) == 0 {
		return nil
	}

	cmd := tokens[0]
	args := tokens[1:]

	h.HTTP.Logger.Debug("ProcessCommand: cmd=%s, args=%v", cmd, args)

	var err error
	switch cmd {
	case "txreq":
		h.HTTP.Logger.Debug("Executing txreq")
		err = h.handleTxReq(args)
	case "txresp":
		h.HTTP.Logger.Debug("Executing txresp")
		err = h.handleTxResp(args)
	case "rxreq":
		h.HTTP.Logger.Debug("Executing rxreq")
		err = h.handleRxReq(args)
	case "rxresp":
		h.HTTP.Logger.Debug("Executing rxresp")
		err = h.handleRxResp(args)
	case "expect":
		h.HTTP.Logger.Debug("Executing expect")
		err = h.handleExpect(args)
	case "send":
		h.HTTP.Logger.Debug("Executing send")
		err = h.handleSend(args)
	case "sendhex":
		h.HTTP.Logger.Debug("Executing sendhex")
		err = h.handleSendHex(args)
	case "recv":
		h.HTTP.Logger.Debug("Executing recv")
		err = h.handleRecv(args)
	case "timeout":
		h.HTTP.Logger.Debug("Executing timeout")
		err = h.handleTimeout(args)
	case "gunzip":
		h.HTTP.Logger.Debug("Executing gunzip")
		err = h.HTTP.Gunzip()
	case "delay":
		h.HTTP.Logger.Debug("Executing delay")
		err = h.handleDelay(args)
	default:
		// Try to execute as a global VTC command
		err = h.tryGlobalCommand(cmd, args)
		if err != nil {
			err = fmt.Errorf("unknown HTTP command: %s", cmd)
		}
	}

	if err != nil {
		h.HTTP.Logger.Debug("Command %s failed: %v", cmd, err)
	} else {
		h.HTTP.Logger.Debug("Command %s completed successfully", cmd)
	}

	return err
}

// tryGlobalCommand attempts to execute a command as a global VTC command
func (h *Handler) tryGlobalCommand(cmd string, args []string) error {
	if h.Context == nil {
		return fmt.Errorf("no context available for global commands")
	}

	// Try to execute as a global VTC command (barrier, shell, delay, etc.)
	h.HTTP.Logger.Debug("Attempting to execute '%s' as global VTC command", cmd)
	err := vtc.ExecuteCommand(cmd, args, h.Context, h.HTTP.Logger)
	if err != nil {
		h.HTTP.Logger.Debug("Global command '%s' failed: %v", cmd, err)
		return err
	}

	h.HTTP.Logger.Debug("Global command '%s' executed successfully", cmd)
	return nil
}

// handleTxReq processes txreq command
func (h *Handler) handleTxReq(args []string) error {
	opts := &TxReqOptions{
		Method: "GET",
		URL:    "/",
		Proto:  "HTTP/1.1",
		Headers: make(map[string]string),
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-method", "-req":
			if i+1 >= len(args) {
				return fmt.Errorf("-method requires an argument")
			}
			opts.Method = args[i+1]
			i++
		case "-url":
			if i+1 >= len(args) {
				return fmt.Errorf("-url requires an argument")
			}
			opts.URL = args[i+1]
			i++
		case "-proto":
			if i+1 >= len(args) {
				return fmt.Errorf("-proto requires an argument")
			}
			opts.Proto = args[i+1]
			i++
		case "-hdr":
			if i+1 >= len(args) {
				return fmt.Errorf("-hdr requires an argument")
			}
			hdr := args[i+1]
			parts := strings.SplitN(hdr, ":", 2)
			if len(parts) == 2 {
				opts.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
			i++
		case "-body":
			if i+1 >= len(args) {
				return fmt.Errorf("-body requires an argument")
			}
			opts.Body = []byte(args[i+1])
			i++
		case "-bodylen":
			if i+1 >= len(args) {
				return fmt.Errorf("-bodylen requires an argument")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid -bodylen: %w", err)
			}
			opts.BodyLen = n
			i++
		case "-bodyfrom":
			if i+1 >= len(args) {
				return fmt.Errorf("-bodyfrom requires an argument")
			}
			filename := args[i+1]
			body, err := h.readBodyFromFile(filename)
			if err != nil {
				return fmt.Errorf("-bodyfrom failed: %w", err)
			}
			opts.Body = body
			i++
		case "-chunked":
			opts.Chunked = true
		case "-gzip":
			opts.Gzip = true
		case "-gzipbody":
			if i+1 >= len(args) {
				return fmt.Errorf("-gzipbody requires an argument")
			}
			opts.Body = []byte(args[i+1])
			opts.Gzip = true
			i++
		case "-nohost":
			opts.NoHost = true
		case "-nouseragent":
			opts.NoUserAgent = true
		default:
			return fmt.Errorf("unknown txreq option: %s", args[i])
		}
	}

	return h.HTTP.TxReq(opts)
}

// handleTxResp processes txresp command
func (h *Handler) handleTxResp(args []string) error {
	opts := &TxRespOptions{
		Status: 200,
		Reason: "OK",
		Proto:  "HTTP/1.1",
		Headers: make(map[string]string),
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-status":
			if i+1 >= len(args) {
				return fmt.Errorf("-status requires an argument")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid -status: %w", err)
			}
			opts.Status = n
			i++
		case "-reason":
			if i+1 >= len(args) {
				return fmt.Errorf("-reason requires an argument")
			}
			opts.Reason = args[i+1]
			i++
		case "-proto":
			if i+1 >= len(args) {
				return fmt.Errorf("-proto requires an argument")
			}
			opts.Proto = args[i+1]
			i++
		case "-hdr":
			if i+1 >= len(args) {
				return fmt.Errorf("-hdr requires an argument")
			}
			hdr := args[i+1]
			parts := strings.SplitN(hdr, ":", 2)
			if len(parts) == 2 {
				opts.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
			i++
		case "-body":
			if i+1 >= len(args) {
				return fmt.Errorf("-body requires an argument")
			}
			opts.Body = []byte(args[i+1])
			i++
		case "-bodylen":
			if i+1 >= len(args) {
				return fmt.Errorf("-bodylen requires an argument")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid -bodylen: %w", err)
			}
			opts.BodyLen = n
			i++
		case "-bodyfrom":
			if i+1 >= len(args) {
				return fmt.Errorf("-bodyfrom requires an argument")
			}
			filename := args[i+1]
			body, err := h.readBodyFromFile(filename)
			if err != nil {
				return fmt.Errorf("-bodyfrom failed: %w", err)
			}
			opts.Body = body
			i++
		case "-chunked":
			opts.Chunked = true
		case "-gzip":
			opts.Gzip = true
		case "-gzipbody":
			if i+1 >= len(args) {
				return fmt.Errorf("-gzipbody requires an argument")
			}
			opts.Body = []byte(args[i+1])
			opts.Gzip = true
			i++
		case "-gziplevel":
			if i+1 >= len(args) {
				return fmt.Errorf("-gziplevel requires an argument")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid -gziplevel: %w", err)
			}
			if n < 0 || n > 9 {
				return fmt.Errorf("-gziplevel must be between 0 and 9")
			}
			h.HTTP.GzipLevel = n
			i++
		case "-nolen":
			opts.NoLen = true
		case "-noserver":
			opts.NoServer = true
		default:
			return fmt.Errorf("unknown txresp option: %s", args[i])
		}
	}

	return h.HTTP.TxResp(opts)
}

// handleRxReq processes rxreq command
func (h *Handler) handleRxReq(args []string) error {
	opts := &RxReqOptions{}
	return h.HTTP.RxReq(opts)
}

// handleRxResp processes rxresp command
func (h *Handler) handleRxResp(args []string) error {
	opts := &RxRespOptions{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-no_obj":
			opts.NoObj = true
		default:
			return fmt.Errorf("unknown rxresp option: %s", args[i])
		}
	}

	return h.HTTP.RxResp(opts)
}

// handleExpect processes expect command
func (h *Handler) handleExpect(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("expect requires at least 3 arguments: field op value")
	}

	field := args[0]
	op := args[1]
	expected := strings.Join(args[2:], " ")

	return h.HTTP.Expect(field, op, expected)
}

// handleSend processes send command
func (h *Handler) handleSend(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("send requires data argument")
	}

	data := strings.Join(args, " ")
	return h.HTTP.SendString(data)
}

// handleSendHex processes sendhex command
func (h *Handler) handleSendHex(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("sendhex requires hex data argument")
	}

	hexStr := strings.Join(args, " ")
	return h.HTTP.SendHex(hexStr)
}

// handleRecv processes recv command
func (h *Handler) handleRecv(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("recv requires byte count argument")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid byte count: %w", err)
	}

	_, err = h.HTTP.Recv(n)
	return err
}

// handleTimeout processes timeout command
func (h *Handler) handleTimeout(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("timeout requires duration argument")
	}

	d, err := time.ParseDuration(args[0])
	if err != nil {
		// Try parsing as seconds
		seconds, err2 := strconv.ParseFloat(args[0], 64)
		if err2 != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
		d = time.Duration(seconds * float64(time.Second))
	}

	h.HTTP.SetIOTimeout(d)
	return nil
}

// handleDelay processes delay command - sleeps for specified duration
func (h *Handler) handleDelay(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("delay requires duration argument")
	}

	d, err := time.ParseDuration(args[0] + "s")
	if err != nil {
		// Try parsing as seconds directly
		seconds, err2 := strconv.ParseFloat(args[0], 64)
		if err2 != nil {
			return fmt.Errorf("invalid delay duration: %w", err)
		}
		d = time.Duration(seconds * float64(time.Second))
	}

	h.HTTP.Logger.Debug("Delaying for %v", d)
	time.Sleep(d)
	return nil
}

// tokenizeCommand splits a command line into tokens
// Handles quoted strings
func tokenizeCommand(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(line); i++ {
		ch := line[i]

		switch {
		case (ch == '"' || ch == '\'') && !inQuote:
			inQuote = true
			quoteChar = ch
		case ch == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
		case (ch == ' ' || ch == '\t') && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// readBodyFromFile reads the body content from a file
func (h *Handler) readBodyFromFile(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	return data, nil
}
