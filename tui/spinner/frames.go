// tui/spinner/frames.go
package spinner

// brailleFrames is the canonical SafeDep spinner animation. Single set, not
// configurable — uniformity is the point.
var brailleFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

const frameInterval = 100 // milliseconds per frame
