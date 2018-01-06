package tunnet

import "encoding/binary"

const (
	pfRoute             = 17
	pfSystem            = 32
	sysprotoControl     = 2
	afSysControl        = 2
	ioctlCTLIOCGINFO    = 0xc0644e03
	ioctlSIOCGIFMTU     = 0xc0206933
	ioctlSIOCSIFMTU     = 0x80206934
	ioctlSIOCGIFADDR    = 0xc0206921
	ioctlSIOCGIFDSTADDR = 0xc0206922
	ioctlSIOCGIFNETMASK = 0xc0206925
	ioctlSIOCDIFADDR    = 0x80206919
	ioctlSIOCAIFADDR    = 0x8040691a
)

var systemByteOrder = binary.LittleEndian
