/*
©AngelaMos | 2026
all.go

Side-effect imports that register every carrier implementation into the registry
*/

package all

import (
	_ "github.com/CarterPerez-dev/crypha/internal/carrier/audio"
	_ "github.com/CarterPerez-dev/crypha/internal/carrier/image"
	_ "github.com/CarterPerez-dev/crypha/internal/carrier/pdf"
	_ "github.com/CarterPerez-dev/crypha/internal/carrier/qr"
	_ "github.com/CarterPerez-dev/crypha/internal/carrier/text"
)
