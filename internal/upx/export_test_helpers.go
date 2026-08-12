package upx

// Exported for local experiments / tests
func Nrv2bExport(src []byte, dstLen int) ([]byte, error) { return nrv2bDecompress(src, dstLen) }
func Nrv2dExport(src []byte, dstLen int) ([]byte, error) { return nrv2dDecompress(src, dstLen) }
func LzmaExport(src []byte, dstLen int) ([]byte, error)  { return upxLZMADecompress(src, dstLen) }
