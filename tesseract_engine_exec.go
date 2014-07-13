package ocrworker

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/couchbaselabs/logg"
)

// This variant of the TesseractEngine calls tesseract via exec rather
// than go.tesseract.
// TODO: update dockerfile to install go-tesseract package!
type TesseractEngineExec struct {
}

/*
Json:

"args":{
  "cFlags":{
    "tessedit_char_whitelist":"0123456789"
  }
}

*/
type TesseractEngineExecArgs struct {
	cFlags map[string]string `json:"c_flags"`
}

/*func NewTesseractEngineExecArgs(ocrRequest OcrRequest) *TesseractEngineExecArgs {

}*/

// return a slice that can be passed to tesseract binary as command line
// args, eg, ["-c", "tessedit_char_whitelist=0123456789", "-c", "foo=bar"]
func (t TesseractEngineExecArgs) ExportCFlags() []string {
	result := []string{}
	for k, v := range t.cFlags {
		result = append(result, "-c")
		keyValArg := fmt.Sprintf("%s=%s", k, v)
		result = append(result, keyValArg)
	}
	return result
}

func (t TesseractEngineExec) ProcessRequest(ocrRequest OcrRequest) (OcrResult, error) {

	cFlagsMapInterfaceOrig := ocrRequest.EngineArgs["c_flags"]

	logg.LogTo("OCR_TESSERACT", "got cFlagsMap: %v type: %T", cFlagsMapInterfaceOrig, cFlagsMapInterfaceOrig)

	cFlagsMapInterface := cFlagsMapInterfaceOrig.(map[string]interface{})

	cFlagsMap := make(map[string]string)
	for k, v := range cFlagsMapInterface {
		v, ok := v.(string)
		if !ok {
			return OcrResult{}, fmt.Errorf("invalid cflag")
		}
		cFlagsMap[k] = v
	}

	logg.LogTo("OCR_TESSERACT", "got cFlagsMap: %v type: %T", cFlagsMap, cFlagsMap)
	engineArgs := TesseractEngineExecArgs{
		cFlags: cFlagsMap,
	}

	tmpFileName, err := func() (string, error) {
		if ocrRequest.ImgUrl != "" {
			return t.tmpFileFromImageUrl(ocrRequest.ImgUrl)
		} else {
			return t.tmpFileFromImageBytes(ocrRequest.ImgBytes)
		}

	}()

	if err != nil {
		logg.LogTo("OCR_TESSERACT", "error getting tmpFileName")
		return OcrResult{}, err
	}

	defer os.Remove(tmpFileName)
	ocrResult, err := t.processImageFile(tmpFileName, engineArgs)

	return ocrResult, err

}

func (t TesseractEngineExec) tmpFileFromImageBytes(imgBytes []byte) (string, error) {

	tmpFileName, err := createTempFileName()
	if err != nil {
		return "", err
	}

	// we have to write the contents of the image url to a temp
	// file, because the leptonica lib can't seem to handle byte arrays
	err = saveBytesToFileName(imgBytes, tmpFileName)
	if err != nil {
		return "", err
	}

	return tmpFileName, nil

}

func (t TesseractEngineExec) tmpFileFromImageUrl(imgUrl string) (string, error) {

	tmpFileName, err := createTempFileName()
	if err != nil {
		return "", err
	}
	// we have to write the contents of the image url to a temp
	// file, because the leptonica lib can't seem to handle byte arrays
	err = saveUrlContentToFileName(imgUrl, tmpFileName)
	if err != nil {
		return "", err
	}

	return tmpFileName, nil

}

func (t TesseractEngineExec) processImageFile(inputFilename string, engineArgs TesseractEngineExecArgs) (OcrResult, error) {

	// give tesseract a unique output filename
	tmpOutFileBaseName, err := createTempFileName()
	if err != nil {
		logg.LogTo("OCR_TESSERACT", "Error creating tmp file: %v", err)
		return OcrResult{}, err
	}

	// the actual file it writes to will have a .txt extension
	tmpOutFileName := fmt.Sprintf("%s.txt", tmpOutFileBaseName)

	// delete output file when we are done
	defer os.Remove(tmpOutFileName)

	// build args array
	cflags := engineArgs.ExportCFlags()
	cmdArgs := []string{inputFilename, tmpOutFileBaseName}
	cmdArgs = append(cmdArgs, cflags...)
	logg.LogTo("OCR_TESSERACT", "cmdArgs: %v", cmdArgs)

	// exec tesseract
	cmd := exec.Command("tesseract", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logg.LogTo("OCR_TESSERACT", "Error exec tesseract: %v %v", err, string(output))
		return OcrResult{}, err
	}

	// get data from outfile
	outBytes, err := ioutil.ReadFile(tmpOutFileName)
	if err != nil {
		logg.LogTo("OCR_TESSERACT", "Error getting data from out file: %v", err)
		return OcrResult{}, err
	}

	return OcrResult{
		Text: string(outBytes),
	}, nil

}