package ncnn

import _ "embed"

//go:embed models/speaker_eres2net.ncnn.param
var speakerERes2NetParam []byte

//go:embed models/speaker_eres2net.ncnn.bin
var speakerERes2NetBin []byte

//go:embed models/vad_silero.ncnn.param
var vadSileroParam []byte

//go:embed models/vad_silero.ncnn.bin
var vadSileroBin []byte

//go:embed models/denoise_nsnet2.ncnn.param
var denoiseNSNet2Param []byte

//go:embed models/denoise_nsnet2.ncnn.bin
var denoiseNSNet2Bin []byte

func init() {
	RegisterModel(ModelSpeakerERes2Net, speakerERes2NetParam, speakerERes2NetBin)
	RegisterModel(ModelVADSilero, vadSileroParam, vadSileroBin)
	RegisterModel(ModelDenoiseNSNet2, denoiseNSNet2Param, denoiseNSNet2Bin)
}
