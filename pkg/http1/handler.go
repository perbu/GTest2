package http1

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Handler processes HTTP command specifications
type Handler struct {
	HTTP *HTTP
}

// NewHandler creates a new HTTP command handler
func NewHandler(h *HTTP) *Handler {
	return &Handler{HTTP: h}
}

// ProcessSpec processes an HTTP command specification string
// This is the main entry point for executing HTTP commands from VTC specs
func (h *Handler) ProcessSpec(spec string) error {
	// Parse the spec into lines
	lines := strings.Split(spec, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the command line
		err := h.ProcessCommand(line)
		if err != nil {
			return fmt.Errorf("command '%s' failed: %w", line, err)
		}
	}

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

	switch cmd {
	case "txreq":
		return h.handleTxReq(args)
	case "txresp":
		return h.handleTxResp(args)
	case "rxreq":
		return h.handleRxReq(args)
	case "rxresp":
		return h.handleRxResp(args)
	case "expect":
		return h.handleExpect(args)
	case "send":
		return h.handleSend(args)
	case "sendhex":
		return h.handleSendHex(args)
	case "recv":
		return h.handleRecv(args)
	case "timeout":
		return h.handleTimeout(args)
	case "gunzip":
		return h.HTTP.Gunzip()
	default:
		return fmt.Errorf("unknown HTTP command: %s", cmd)
	}
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
		case "-chunked":
			opts.Chunked = true
		case "-gzip":
			opts.Gzip = true
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
		case "-chunked":
			opts.Chunked = true
		case "-gzip":
			opts.Gzip = true
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
