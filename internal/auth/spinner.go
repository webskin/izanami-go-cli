package auth

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// Spinner displays an animated progress indicator during long-running operations.
//
// # Terminal Compatibility
//
// The spinner only animates when writing to a terminal that supports ANSI escape
// codes. When writing to a non-terminal (e.g., file, pipe), it displays a static
// message without animation.
//
// # Usage
//
//	spinner := auth.NewSpinner(os.Stderr, "Waiting for authentication")
//	spinner.Start()
//	// ... perform operation ...
//	spinner.Success("Authentication complete!")
//
// # Thread Safety
//
// Spinner is safe for concurrent use. Start, Stop, Success, and Error can be
// called from different goroutines.
type Spinner struct {
	// frames are the animation frames (braille pattern spinner)
	frames []string

	// message is displayed next to the spinner
	message string

	// writer is where output is written (typically os.Stderr)
	writer io.Writer

	// interval is the time between animation frames
	interval time.Duration

	// stopChan signals the animation goroutine to stop
	stopChan chan struct{}

	// doneChan signals that the animation goroutine has stopped
	doneChan chan struct{}

	// isaTTY indicates if the writer is a terminal
	isaTTY bool

	// mu protects concurrent access
	mu sync.Mutex

	// running indicates if the spinner is currently animating
	running bool
}

// Default spinner configuration
const (
	// DefaultSpinnerInterval is the time between animation frames.
	// 100ms provides smooth animation without excessive CPU usage.
	DefaultSpinnerInterval = 100 * time.Millisecond
)

// spinnerFrames are the braille pattern characters used for animation.
// These provide a smooth circular animation effect.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner creates a new Spinner that writes to the given writer.
//
// Parameters:
//   - w: Where to write output (typically os.Stderr for CLI tools)
//   - message: The message to display next to the spinner
//
// The spinner automatically detects if the writer is a terminal and
// adjusts its behavior accordingly (animated vs static).
//
// Example:
//
//	spinner := auth.NewSpinner(os.Stderr, "Loading")
//	spinner.Start()
//	defer spinner.Stop()
func NewSpinner(w io.Writer, message string) *Spinner {
	return &Spinner{
		frames:   spinnerFrames,
		message:  message,
		writer:   w,
		interval: DefaultSpinnerInterval,
		isaTTY:   isTerminal(w),
	}
}

// isTerminal checks if the writer is a terminal that supports ANSI escape codes.
//
// This is used to determine whether to animate the spinner or display
// a static message. Animation uses ANSI escape codes to clear and
// rewrite the line, which doesn't work in non-terminal contexts.
func isTerminal(w io.Writer) bool {
	// Check if writer is a file
	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	// Check if file is a terminal
	return term.IsTerminal(int(f.Fd()))
}

// Start begins the spinner animation.
//
// If the writer is a terminal, this starts the animation loop in a
// separate goroutine. The animation continues until Stop, Success,
// or Error is called.
//
// If the writer is not a terminal, this simply prints the message
// once without animation.
//
// Calling Start on an already running spinner has no effect.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true
	s.stopChan = make(chan struct{})
	s.doneChan = make(chan struct{})

	if s.isaTTY {
		// Start animation in background goroutine
		go s.animate()
	} else {
		// Non-terminal: just print the message once
		fmt.Fprintf(s.writer, "%s...\n", s.message)
	}
}

// animate runs the spinner animation loop.
// This should only be called from Start() in a goroutine.
func (s *Spinner) animate() {
	defer close(s.doneChan)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	frameIndex := 0

	// Print initial frame
	s.printFrame(frameIndex)

	for {
		select {
		case <-s.stopChan:
			// Clear the spinner line before returning
			s.clearLine()
			return
		case <-ticker.C:
			frameIndex = (frameIndex + 1) % len(s.frames)
			s.printFrame(frameIndex)
		}
	}
}

// printFrame prints a single animation frame.
// Uses ANSI escape codes to overwrite the current line.
func (s *Spinner) printFrame(index int) {
	// \r moves cursor to beginning of line
	// This allows us to overwrite the previous frame
	fmt.Fprintf(s.writer, "\r%s %s... ", s.frames[index], s.message)
}

// clearLine clears the current line using ANSI escape codes.
func (s *Spinner) clearLine() {
	// \r moves cursor to beginning of line
	// \033[K clears from cursor to end of line
	fmt.Fprint(s.writer, "\r\033[K")
}

// Stop stops the spinner animation without displaying a final message.
//
// This clears the spinner line, leaving the terminal clean.
// Use Success or Error if you want to display a final status message.
//
// Calling Stop on a stopped spinner has no effect.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	s.running = false
	close(s.stopChan)
	s.mu.Unlock()

	// Wait for animation goroutine to finish (if it was running)
	if s.isaTTY {
		<-s.doneChan
	}
}

// Success stops the spinner and displays a success message.
//
// The spinner line is replaced with a green checkmark and the provided
// message. This is typically used when the operation completes successfully.
//
// Example:
//
//	spinner.Success("Authentication complete!")
//	// Output: ✅ Authentication complete!
func (s *Spinner) Success(message string) {
	s.Stop()

	if s.isaTTY {
		// Green checkmark for success
		fmt.Fprintf(s.writer, "\r✅ %s\n", message)
	} else {
		fmt.Fprintf(s.writer, "Success: %s\n", message)
	}
}

// Error stops the spinner and displays an error message.
//
// The spinner line is replaced with a red X and the provided message.
// This is typically used when the operation fails.
//
// Example:
//
//	spinner.Error("Authentication timed out")
//	// Output: ❌ Authentication timed out
func (s *Spinner) Error(message string) {
	s.Stop()

	if s.isaTTY {
		// Red X for error
		fmt.Fprintf(s.writer, "\r❌ %s\n", message)
	} else {
		fmt.Fprintf(s.writer, "Error: %s\n", message)
	}
}

// UpdateMessage changes the message displayed next to the spinner.
//
// This can be used to update progress information while the spinner
// is running.
//
// Example:
//
//	spinner.Start()
//	spinner.UpdateMessage("Step 1 of 3")
//	// ... do step 1 ...
//	spinner.UpdateMessage("Step 2 of 3")
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}
