// internal/v2/container/envelope_v2.go 信封层
package envelope

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// ReadEnvelopeFooter_v2 从文件末尾读取 Footer
func ReadEnvelopeFooter_v2(r io.ReadSeeker) (*types.EnvelopeFooter_v2, error) {
	var footer types.EnvelopeFooter_v2

	// Seek to the beginning of the footer
	_, err := r.Seek(-int64(types.EnvelopeFooterSize_v2), io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to footer: %w", err)
	}

	err = binary.Read(r, types.ByteOrder_v2, &footer)
	if err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}

	if !bytes.Equal(footer.Magic[:], types.MagicFooter_v2[:]) {
		return nil, types.ErrInvalidMagic_v2
	}

	return &footer, nil
}
