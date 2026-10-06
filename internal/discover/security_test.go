package discover

import "testing"

func TestParseSecurity(t *testing.T) {
	virgin := `
SPI_BOOT_CRYPT_CNT (BLOCK0)   Enables flash encryption when 1 or 3 bits are set  = Disable R/W (0b000)
SECURE_BOOT_EN (BLOCK0)       Set this bit to enable secure boot                 = False R/W (0b0)
`
	if s := parseSecurity(virgin); s.FlashEncryption || s.SecureBoot {
		t.Errorf("virgin chip read as protected: %+v", s)
	}

	locked := `
SPI_BOOT_CRYPT_CNT (BLOCK0)   Enables flash encryption when 1 or 3 bits are set  = Enable R/W (0b001)
SECURE_BOOT_EN (BLOCK0)       Set this bit to enable secure boot                 = True R/W (0b1)
`
	if s := parseSecurity(locked); !s.FlashEncryption || !s.SecureBoot {
		t.Errorf("locked chip not detected: %+v", s)
	}

	feOnly := `
SPI_BOOT_CRYPT_CNT (BLOCK0)   ...  = Enable R/W (0b001)
SECURE_BOOT_EN (BLOCK0)       ...  = False R/W (0b0)
`
	if s := parseSecurity(feOnly); !s.FlashEncryption || s.SecureBoot {
		t.Errorf("FE-only chip misread: %+v", s)
	}
}
