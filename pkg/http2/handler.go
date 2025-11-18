package http2

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/perbu/GTest/pkg/hpack"
)

// Handler processes HTTP/2 command specifications
type Handler struct {
	Conn          *Conn
	activeStreams map[uint32]*StreamContext
	streamsMu     sync.Mutex
}

// StreamContext holds execution context for a stream
type StreamContext struct {
	StreamID  uint32
	WaitGroup sync.WaitGroup
	Error     error
}

// NewHandler creates a new HTTP/2 command handler
func NewHandler(conn *Conn) *Handler {
	return &Handler{
		Conn:          conn,
		activeStreams: make(map[uint32]*StreamContext),
	}
}

// ProcessSpec processes an HTTP/2 command specification string
// This is the main entry point for executing HTTP/2 commands from VTC specs
func (h *Handler) ProcessSpec(spec string) error {
	h.Conn.logger.Debug("HTTP/2 ProcessSpec called with spec length: %d", len(spec))

	// Parse the spec into lines
	lines := strings.Split(spec, "\n")
	h.Conn.logger.Debug("HTTP/2 ProcessSpec parsed %d lines", len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		h.Conn.logger.Debug("Processing line %d: %s", i+1, line)

		// Parse the command line
		err := h.ProcessCommand(line)
		if err != nil {
			h.Conn.logger.Debug("Command failed on line %d: %v", i+1, err)
			return fmt.Errorf("command '%s' failed: %w", line, err)
		}

		h.Conn.logger.Debug("Line %d completed successfully", i+1)
	}

	h.Conn.logger.Debug("HTTP/2 ProcessSpec completed successfully")
	return nil
}

// ProcessCommand processes a single HTTP/2 command
func (h *Handler) ProcessCommand(cmdLine string) error {
	// Tokenize the command line
	tokens := tokenizeCommand(cmdLine)
	if len(tokens) == 0 {
		return nil
	}

	cmd := tokens[0]
	args := tokens[1:]

	h.Conn.logger.Debug("ProcessCommand: cmd=%s, args=%v", cmd, args)

	var err error
	switch cmd {
	case "stream":
		h.Conn.logger.Debug("Executing stream command")
		err = h.handleStream(args)
	case "txpri":
		h.Conn.logger.Debug("Executing txpri")
		err = h.Conn.TxPri()
	case "rxpri":
		h.Conn.logger.Debug("Executing rxpri")
		err = h.Conn.RxPri()
	case "txsettings":
		h.Conn.logger.Debug("Executing txsettings")
		err = h.handleTxSettings(args)
	case "rxsettings":
		h.Conn.logger.Debug("Executing rxsettings")
		_, err = h.Conn.RxSettings()
	case "sendhex":
		h.Conn.logger.Debug("Executing sendhex")
		err = h.handleSendHex(args)
	case "delay":
		h.Conn.logger.Debug("Executing delay")
		err = h.handleDelay(args)
	default:
		err = fmt.Errorf("unknown HTTP/2 command: %s", cmd)
	}

	if err != nil {
		h.Conn.logger.Debug("Command %s failed: %v", cmd, err)
	} else {
		h.Conn.logger.Debug("Command %s completed successfully", cmd)
	}

	return err
}

// ProcessStreamCommand processes a command in the context of a specific stream
func (h *Handler) ProcessStreamCommand(streamID uint32, cmdLine string) error {
	// Tokenize the command line
	tokens := tokenizeCommand(cmdLine)
	if len(tokens) == 0 {
		return nil
	}

	cmd := tokens[0]
	args := tokens[1:]

	h.Conn.logger.Debug("ProcessStreamCommand: stream=%d, cmd=%s, args=%v", streamID, cmd, args)

	var err error
	switch cmd {
	case "txreq":
		h.Conn.logger.Debug("Executing txreq on stream %d", streamID)
		err = h.handleTxReq(streamID, args)
	case "txresp":
		h.Conn.logger.Debug("Executing txresp on stream %d", streamID)
		err = h.handleTxResp(streamID, args)
	case "rxreq":
		h.Conn.logger.Debug("Executing rxreq on stream %d", streamID)
		err = h.Conn.RxReq(streamID)
	case "rxresp":
		h.Conn.logger.Debug("Executing rxresp on stream %d", streamID)
		err = h.Conn.RxResp(streamID)
	case "txdata":
		h.Conn.logger.Debug("Executing txdata on stream %d", streamID)
		err = h.handleTxData(streamID, args)
	case "rxdata":
		h.Conn.logger.Debug("Executing rxdata on stream %d", streamID)
		_, err = h.Conn.RxData(streamID)
	case "rxhdrs":
		h.Conn.logger.Debug("Executing rxhdrs on stream %d", streamID)
		// rxhdrs is implicitly handled by rxreq/rxresp
		// Just wait for headers to arrive
		stream, ok := h.Conn.GetStream(streamID)
		if !ok {
			err = fmt.Errorf("stream %d not found", streamID)
		} else {
			stream.Wait()
		}
	case "txprio":
		h.Conn.logger.Debug("Executing txprio on stream %d", streamID)
		err = h.handleTxPrio(streamID, args)
	case "txrst":
		h.Conn.logger.Debug("Executing txrst on stream %d", streamID)
		err = h.handleTxRst(streamID, args)
	case "txping":
		h.Conn.logger.Debug("Executing txping on stream %d", streamID)
		err = h.handleTxPing(streamID, args)
	case "rxprio":
		h.Conn.logger.Debug("Executing rxprio on stream %d", streamID)
		// rxprio receives a PRIORITY frame - handled by frame loop, just store it
		// For now, we'll need to wait for the frame and store it for expect
		// This is a TODO - need to implement frame storage for expectations
		err = nil
	case "rxrst":
		h.Conn.logger.Debug("Executing rxrst on stream %d", streamID)
		err = h.Conn.RxRst(streamID)
	case "rxping":
		h.Conn.logger.Debug("Executing rxping on stream %d", streamID)
		_, err = h.Conn.RxPing()
	case "txgoaway":
		h.Conn.logger.Debug("Executing txgoaway on stream %d", streamID)
		err = h.handleTxGoAway(streamID, args)
	case "rxgoaway":
		h.Conn.logger.Debug("Executing rxgoaway on stream %d", streamID)
		err = h.Conn.RxGoAway()
	case "txwinup":
		h.Conn.logger.Debug("Executing txwinup on stream %d", streamID)
		err = h.handleTxWinup(streamID, args)
	case "rxwinup":
		h.Conn.logger.Debug("Executing rxwinup on stream %d", streamID)
		_, err = h.Conn.RxWinup(streamID)
	case "expect":
		h.Conn.logger.Debug("Executing expect on stream %d", streamID)
		err = h.handleExpect(streamID, args)
	case "sendhex":
		h.Conn.logger.Debug("Executing sendhex on stream %d", streamID)
		err = h.handleSendHex(args)
	case "delay":
		h.Conn.logger.Debug("Executing delay")
		err = h.handleDelay(args)
	default:
		err = fmt.Errorf("unknown HTTP/2 stream command: %s", cmd)
	}

	if err != nil {
		h.Conn.logger.Debug("Stream command %s failed: %v", cmd, err)
	} else {
		h.Conn.logger.Debug("Stream command %s completed successfully", cmd)
	}

	return err
}

// handleStream processes the stream command
// Syntax: stream ID { commands... } -run|-start|-wait
func (h *Handler) handleStream(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("stream: requires stream ID and spec or flags")
	}

	// Parse stream ID
	streamID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		return fmt.Errorf("stream: invalid stream ID: %w", err)
	}

	// Look for flags and collect spec parts
	var specParts []string
	var runMode string // "run", "start", or "wait"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-run":
			runMode = "run"
		case "-start":
			runMode = "start"
		case "-wait":
			runMode = "wait"
		default:
			// This is part of the spec
			specParts = append(specParts, args[i])
		}
	}

	// Handle -wait flag (wait for stream to complete)
	if runMode == "wait" {
		return h.waitForStream(uint32(streamID))
	}

	// Join spec parts with spaces (they have been split by the tokenizer)
	// Also handle the special ||| delimiter used for nested commands
	spec := strings.Join(specParts, " ")
	spec = strings.ReplaceAll(spec, "|||", "\n")

	// For -run and -start, we need a spec
	if spec == "" {
		return fmt.Errorf("stream: no spec provided")
	}

	// Execute stream spec
	if runMode == "start" {
		return h.startStream(uint32(streamID), spec)
	}

	// Default to -run (synchronous execution)
	return h.runStream(uint32(streamID), spec)
}

// runStream executes a stream spec synchronously
func (h *Handler) runStream(streamID uint32, spec string) error {
	h.Conn.logger.Debug("Running stream %d synchronously", streamID)

	// Parse the spec into lines
	lines := strings.Split(spec, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		h.Conn.logger.Debug("Stream %d line %d: %s", streamID, i+1, line)

		// Execute the command in the stream context
		err := h.ProcessStreamCommand(streamID, line)
		if err != nil {
			h.Conn.logger.Debug("Stream %d command failed on line %d: %v", streamID, i+1, err)
			return fmt.Errorf("stream %d command '%s' failed: %w", streamID, line, err)
		}
	}

	h.Conn.logger.Debug("Stream %d completed successfully", streamID)
	return nil
}

// startStream executes a stream spec asynchronously in a goroutine
func (h *Handler) startStream(streamID uint32, spec string) error {
	h.Conn.logger.Debug("Starting stream %d asynchronously", streamID)

	// Create stream context
	streamCtx := &StreamContext{
		StreamID: streamID,
	}

	h.streamsMu.Lock()
	h.activeStreams[streamID] = streamCtx
	h.streamsMu.Unlock()

	// Start goroutine to execute stream
	streamCtx.WaitGroup.Add(1)
	go func() {
		defer streamCtx.WaitGroup.Done()

		err := h.runStream(streamID, spec)
		if err != nil {
			h.Conn.logger.Error("Stream %d failed: %v", streamID, err)
			streamCtx.Error = err
		}
	}()

	return nil
}

// waitForStream waits for a stream started with -start to complete
func (h *Handler) waitForStream(streamID uint32) error {
	h.streamsMu.Lock()
	streamCtx, ok := h.activeStreams[streamID]
	h.streamsMu.Unlock()

	if !ok {
		return fmt.Errorf("stream %d not found or not started", streamID)
	}

	h.Conn.logger.Debug("Waiting for stream %d to complete", streamID)
	streamCtx.WaitGroup.Wait()

	// Clean up
	h.streamsMu.Lock()
	delete(h.activeStreams, streamID)
	h.streamsMu.Unlock()

	if streamCtx.Error != nil {
		return fmt.Errorf("stream %d failed: %w", streamID, streamCtx.Error)
	}

	h.Conn.logger.Debug("Stream %d completed successfully", streamID)
	return nil
}

// tokenizeCommand splits a command line into tokens
// Handles quoted strings and basic tokenization
func tokenizeCommand(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	escaped := false

	for _, ch := range line {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}

		switch ch {
		case '\\':
			escaped = true
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				current.WriteRune(ch)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// Helper command handlers

func (h *Handler) handleDelay(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("delay: missing duration")
	}

	// Parse duration (in seconds unless unit specified)
	durationStr := args[0]
	if !strings.Contains(durationStr, "s") && !strings.Contains(durationStr, "m") {
		durationStr += "s" // Default to seconds
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		// Try parsing as float seconds
		if seconds, err := strconv.ParseFloat(args[0], 64); err == nil {
			duration = time.Duration(seconds * float64(time.Second))
		} else {
			return fmt.Errorf("delay: invalid duration: %s", args[0])
		}
	}

	h.Conn.logger.Debug("Delaying for %v", duration)
	time.Sleep(duration)
	return nil
}

func (h *Handler) handleSendHex(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("sendhex: missing hex data")
	}

	hexData := strings.Join(args, " ")
	return h.Conn.SendHex(hexData)
}

func (h *Handler) handleTxSettings(args []string) error {
	settings := make(map[SettingID]uint32)
	ack := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-ack":
			ack = true
		case "-push":
			if i+1 >= len(args) {
				return fmt.Errorf("txsettings: -push requires a value")
			}
			i++
			val, err := parseBool(args[i])
			if err != nil {
				return fmt.Errorf("txsettings: invalid -push value: %w", err)
			}
			if val {
				settings[SettingEnablePush] = 1
			} else {
				settings[SettingEnablePush] = 0
			}
		case "-hdrtbl":
			if i+1 >= len(args) {
				return fmt.Errorf("txsettings: -hdrtbl requires a value")
			}
			i++
			val, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return fmt.Errorf("txsettings: invalid -hdrtbl value: %w", err)
			}
			settings[SettingHeaderTableSize] = uint32(val)
		case "-maxstreams":
			if i+1 >= len(args) {
				return fmt.Errorf("txsettings: -maxstreams requires a value")
			}
			i++
			val, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return fmt.Errorf("txsettings: invalid -maxstreams value: %w", err)
			}
			settings[SettingMaxConcurrentStreams] = uint32(val)
		case "-winsize":
			if i+1 >= len(args) {
				return fmt.Errorf("txsettings: -winsize requires a value")
			}
			i++
			val, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return fmt.Errorf("txsettings: invalid -winsize value: %w", err)
			}
			settings[SettingInitialWindowSize] = uint32(val)
		case "-framesize":
			if i+1 >= len(args) {
				return fmt.Errorf("txsettings: -framesize requires a value")
			}
			i++
			val, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return fmt.Errorf("txsettings: invalid -framesize value: %w", err)
			}
			settings[SettingMaxFrameSize] = uint32(val)
		case "-hdrsize":
			if i+1 >= len(args) {
				return fmt.Errorf("txsettings: -hdrsize requires a value")
			}
			i++
			val, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return fmt.Errorf("txsettings: invalid -hdrsize value: %w", err)
			}
			settings[SettingMaxHeaderListSize] = uint32(val)
		}
	}

	return h.Conn.TxSettings(ack, settings)
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
}

func (h *Handler) handleTxReq(streamID uint32, args []string) error {
	opts := TxReqOptions{
		Method:            "GET",
		Path:              "/",
		Scheme:            "http",
		Authority:         "localhost",
		Headers:           make(map[string]string),
		EndStream:         true,
		HpackInstructions: nil,
	}

	var hpackInstructions []hpack.HpackInstruction

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-method", "-req":
			if i+1 >= len(args) {
				return fmt.Errorf("txreq: -method requires an argument")
			}
			opts.Method = args[i+1]
			i++
		case "-url":
			if i+1 >= len(args) {
				return fmt.Errorf("txreq: -url requires an argument")
			}
			opts.Path = args[i+1]
			i++
		case "-scheme":
			if i+1 >= len(args) {
				return fmt.Errorf("txreq: -scheme requires an argument")
			}
			opts.Scheme = args[i+1]
			i++
		case "-hdr":
			if i+1 >= len(args) {
				return fmt.Errorf("txreq: -hdr requires an argument")
			}
			hdr := args[i+1]
			parts := strings.SplitN(hdr, ":", 2)
			if len(parts) == 2 {
				opts.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
			i++
		case "-body":
			if i+1 >= len(args) {
				return fmt.Errorf("txreq: -body requires an argument")
			}
			opts.Body = []byte(args[i+1])
			i++
		case "-nostrend":
			opts.EndStream = false
		case "-idxHdr":
			// Indexed header field
			if i+1 >= len(args) {
				return fmt.Errorf("txreq: -idxHdr requires an argument")
			}
			index, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("txreq: -idxHdr: invalid index: %w", err)
			}
			hpackInstructions = append(hpackInstructions, hpack.HpackInstruction{
				Type:  "indexed",
				Index: index,
			})
			i++
		case "-litIdxHdr":
			// Literal header with indexed name
			// Syntax: -litIdxHdr inc|not|never <name-index> huf|plain <value>
			if i+4 >= len(args) {
				return fmt.Errorf("txreq: -litIdxHdr requires 4 arguments: mode nameIndex encoding value")
			}
			indexingMode, err := parseIndexingMode(args[i+1])
			if err != nil {
				return fmt.Errorf("txreq: -litIdxHdr: %w", err)
			}
			nameIndex, err := strconv.Atoi(args[i+2])
			if err != nil {
				return fmt.Errorf("txreq: -litIdxHdr: invalid name index: %w", err)
			}
			valueHuffman := args[i+3] == "huf"
			value := args[i+4]
			hpackInstructions = append(hpackInstructions, hpack.HpackInstruction{
				Type:         "literal-indexed",
				Index:        nameIndex,
				Value:        value,
				IndexingMode: indexingMode,
				ValueHuffman: valueHuffman,
			})
			i += 4
		case "-litHdr":
			// Literal header with new name
			// Syntax: -litHdr inc|not|never huf|plain <name> huf|plain <value>
			if i+5 >= len(args) {
				return fmt.Errorf("txreq: -litHdr requires 5 arguments: mode nameEncoding name valueEncoding value")
			}
			indexingMode, err := parseIndexingMode(args[i+1])
			if err != nil {
				return fmt.Errorf("txreq: -litHdr: %w", err)
			}
			nameHuffman := args[i+2] == "huf"
			name := args[i+3]
			valueHuffman := args[i+4] == "huf"
			value := args[i+5]
			hpackInstructions = append(hpackInstructions, hpack.HpackInstruction{
				Type:         "literal-new",
				Name:         name,
				Value:        value,
				IndexingMode: indexingMode,
				NameHuffman:  nameHuffman,
				ValueHuffman: valueHuffman,
			})
			i += 5
		}
	}

	// If we have HPACK instructions, use them
	if len(hpackInstructions) > 0 {
		opts.HpackInstructions = hpackInstructions
	}

	return h.Conn.TxReq(streamID, opts)
}

// parseIndexingMode parses the indexing mode string
func parseIndexingMode(mode string) (hpack.IndexingMode, error) {
	switch mode {
	case "inc":
		return hpack.IndexingInc, nil
	case "not":
		return hpack.IndexingNot, nil
	case "never":
		return hpack.IndexingNever, nil
	default:
		return 0, fmt.Errorf("invalid indexing mode: %s (expected inc|not|never)", mode)
	}
}

func (h *Handler) handleTxResp(streamID uint32, args []string) error {
	opts := TxRespOptions{
		Status:            "200",
		Headers:           make(map[string]string),
		EndStream:         true,
		HpackInstructions: nil,
	}

	var hpackInstructions []hpack.HpackInstruction

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-status":
			if i+1 >= len(args) {
				return fmt.Errorf("txresp: -status requires an argument")
			}
			opts.Status = args[i+1]
			i++
		case "-hdr":
			if i+1 >= len(args) {
				return fmt.Errorf("txresp: -hdr requires an argument")
			}
			hdr := args[i+1]
			parts := strings.SplitN(hdr, ":", 2)
			if len(parts) == 2 {
				opts.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
			i++
		case "-body":
			if i+1 >= len(args) {
				return fmt.Errorf("txresp: -body requires an argument")
			}
			opts.Body = []byte(args[i+1])
			i++
		case "-nostrend":
			opts.EndStream = false
		case "-idxHdr":
			// Indexed header field
			if i+1 >= len(args) {
				return fmt.Errorf("txresp: -idxHdr requires an argument")
			}
			index, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("txresp: -idxHdr: invalid index: %w", err)
			}
			hpackInstructions = append(hpackInstructions, hpack.HpackInstruction{
				Type:  "indexed",
				Index: index,
			})
			i++
		case "-litIdxHdr":
			// Literal header with indexed name
			// Syntax: -litIdxHdr inc|not|never <name-index> huf|plain <value>
			if i+4 >= len(args) {
				return fmt.Errorf("txresp: -litIdxHdr requires 4 arguments: mode nameIndex encoding value")
			}
			indexingMode, err := parseIndexingMode(args[i+1])
			if err != nil {
				return fmt.Errorf("txresp: -litIdxHdr: %w", err)
			}
			nameIndex, err := strconv.Atoi(args[i+2])
			if err != nil {
				return fmt.Errorf("txresp: -litIdxHdr: invalid name index: %w", err)
			}
			valueHuffman := args[i+3] == "huf"
			value := args[i+4]
			hpackInstructions = append(hpackInstructions, hpack.HpackInstruction{
				Type:         "literal-indexed",
				Index:        nameIndex,
				Value:        value,
				IndexingMode: indexingMode,
				ValueHuffman: valueHuffman,
			})
			i += 4
		case "-litHdr":
			// Literal header with new name
			// Syntax: -litHdr inc|not|never huf|plain <name> huf|plain <value>
			if i+5 >= len(args) {
				return fmt.Errorf("txresp: -litHdr requires 5 arguments: mode nameEncoding name valueEncoding value")
			}
			indexingMode, err := parseIndexingMode(args[i+1])
			if err != nil {
				return fmt.Errorf("txresp: -litHdr: %w", err)
			}
			nameHuffman := args[i+2] == "huf"
			name := args[i+3]
			valueHuffman := args[i+4] == "huf"
			value := args[i+5]
			hpackInstructions = append(hpackInstructions, hpack.HpackInstruction{
				Type:         "literal-new",
				Name:         name,
				Value:        value,
				IndexingMode: indexingMode,
				NameHuffman:  nameHuffman,
				ValueHuffman: valueHuffman,
			})
			i += 5
		}
	}

	// If we have HPACK instructions, use them
	if len(hpackInstructions) > 0 {
		opts.HpackInstructions = hpackInstructions
	}

	return h.Conn.TxResp(streamID, opts)
}

func (h *Handler) handleTxData(streamID uint32, args []string) error {
	var data []byte
	endStream := true

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-data":
			if i+1 >= len(args) {
				return fmt.Errorf("txdata: -data requires an argument")
			}
			data = []byte(args[i+1])
			i++
		case "-nostrend":
			endStream = false
		default:
			// Treat as data
			data = []byte(args[i])
		}
	}

	return h.Conn.TxData(streamID, data, endStream)
}

func (h *Handler) handleTxPrio(streamID uint32, args []string) error {
	var exclusive bool
	var dependsOn uint32
	var weight uint8 = 16 // Default weight

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-stream":
			if i+1 >= len(args) {
				return fmt.Errorf("txprio: -stream requires an argument")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return fmt.Errorf("txprio: invalid -stream value: %w", err)
			}
			dependsOn = uint32(val)
			i++
		case "-weight":
			if i+1 >= len(args) {
				return fmt.Errorf("txprio: -weight requires an argument")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 8)
			if err != nil {
				return fmt.Errorf("txprio: invalid -weight value: %w", err)
			}
			weight = uint8(val)
			i++
		case "-excl":
			exclusive = true
		}
	}

	return h.Conn.TxPriority(streamID, exclusive, dependsOn, weight)
}

func (h *Handler) handleTxRst(streamID uint32, args []string) error {
	var errorCode uint32 = 0 // NO_ERROR

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-err":
			if i+1 >= len(args) {
				return fmt.Errorf("txrst: -err requires an argument")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return fmt.Errorf("txrst: invalid -err value: %w", err)
			}
			errorCode = uint32(val)
			i++
		}
	}

	return h.Conn.TxRst(streamID, errorCode)
}

func (h *Handler) handleTxPing(streamID uint32, args []string) error {
	var data [8]byte
	ack := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-data":
			if i+1 >= len(args) {
				return fmt.Errorf("txping: -data requires an argument")
			}
			dataStr := args[i+1]
			if len(dataStr) > 8 {
				dataStr = dataStr[:8]
			}
			copy(data[:], dataStr)
			i++
		case "-ack":
			ack = true
		}
	}

	return h.Conn.TxPing(ack, data)
}

func (h *Handler) handleTxGoAway(streamID uint32, args []string) error {
	var lastStreamID uint32
	var errorCode uint32
	var debugData string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-laststream":
			if i+1 >= len(args) {
				return fmt.Errorf("txgoaway: -laststream requires an argument")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return fmt.Errorf("txgoaway: invalid -laststream value: %w", err)
			}
			lastStreamID = uint32(val)
			i++
		case "-err":
			if i+1 >= len(args) {
				return fmt.Errorf("txgoaway: -err requires an argument")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return fmt.Errorf("txgoaway: invalid -err value: %w", err)
			}
			errorCode = uint32(val)
			i++
		case "-debug":
			if i+1 >= len(args) {
				return fmt.Errorf("txgoaway: -debug requires an argument")
			}
			debugData = args[i+1]
			i++
		}
	}

	return h.Conn.TxGoAway(lastStreamID, errorCode, debugData)
}

func (h *Handler) handleTxWinup(streamID uint32, args []string) error {
	var size uint32 = 1

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-size":
			if i+1 >= len(args) {
				return fmt.Errorf("txwinup: -size requires an argument")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return fmt.Errorf("txwinup: invalid -size value: %w", err)
			}
			size = uint32(val)
			i++
		}
	}

	return h.Conn.TxWinup(streamID, size)
}

func (h *Handler) handleExpect(streamID uint32, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("expect: requires at least 3 arguments: field op value")
	}

	field := args[0]
	op := args[1]
	expected := strings.Join(args[2:], " ")

	// Handle special cases for stream-specific fields
	if streamID == 0 {
		// Stream 0 context - handle connection-level expectations
		return h.handleConnectionExpect(field, op, expected)
	}

	// Stream-level expectations
	return h.Conn.Expect(streamID, field, op, expected)
}

func (h *Handler) handleConnectionExpect(field, op, expected string) error {
	// Handle connection-level expectations (settings, ping, goaway, winup, prio, rst, frame)
	parts := strings.Split(field, ".")
	if len(parts) < 2 {
		return fmt.Errorf("expect: invalid field format: %s", field)
	}

	// For now, implement basic frame field expectations
	// The actual implementation would need to store received frames for validation
	h.Conn.logger.Debug("Connection-level expect: %s %s %s", field, op, expected)

	// TODO: Implement proper connection-level expectations
	// This would require storing received SETTINGS, PING, GOAWAY frames

	return nil
}
