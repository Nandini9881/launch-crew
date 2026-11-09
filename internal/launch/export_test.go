package launch

import "context"

// Test hooks for the external test package.
func ShrinkToJPEG(data []byte) ([]byte, error) { return shrinkToJPEG(context.Background(), data) }
