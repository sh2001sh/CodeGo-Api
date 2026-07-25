package filex

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"

	"github.com/abema/go-mp4"
	"github.com/go-audio/aiff"
	"github.com/go-audio/wav"
	"github.com/jfreymuth/oggvorbis"
	"github.com/mewkiz/flac"
	"github.com/pkg/errors"
	"github.com/tcolgate/mp3"
	"github.com/yapingcat/gomedia/go-codec"
)

// GetAudioDuration returns the audio duration in seconds using pure Go decoders.
func GetAudioDuration(ctx context.Context, f io.ReadSeeker, ext string) (duration float64, err error) {
	_ = ctx
	log.Printf("GetAudioDuration: ext=%s", ext)
	switch ext {
	case ".mp3":
		duration, err = getMP3Duration(f)
	case ".wav":
		duration, err = getWAVDuration(f)
	case ".flac":
		duration, err = getFLACDuration(f)
	case ".m4a", ".mp4":
		duration, err = getM4ADuration(f)
	case ".ogg", ".oga", ".opus":
		duration, err = getOGGDuration(f)
		if err != nil {
			duration, err = getOpusDuration(f)
		}
	case ".aiff", ".aif", ".aifc":
		duration, err = getAIFFDuration(f)
	case ".webm":
		duration, err = getWebMDuration(f)
	case ".aac":
		duration, err = getAACDuration(f)
	default:
		return 0, fmt.Errorf("unsupported audio format: %s", ext)
	}
	log.Printf("GetAudioDuration: duration=%f", duration)
	return duration, err
}

func getMP3Duration(r io.Reader) (float64, error) {
	d := mp3.NewDecoder(r)
	var f mp3.Frame
	skipped := 0
	duration := 0.0

	for {
		if err := d.Decode(&f, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			return 0, errors.Wrap(err, "failed to decode mp3 frame")
		}
		duration += f.Duration().Seconds()
	}
	return duration, nil
}

func getWAVDuration(r io.ReadSeeker) (float64, error) {
	r.Seek(0, io.SeekStart)

	dec := wav.NewDecoder(r)
	if !dec.IsValidFile() {
		return 0, errors.New("invalid wav file")
	}

	if err := dec.FwdToPCM(); err != nil {
		return 0, errors.Wrap(err, "failed to find PCM data chunk")
	}

	pcmSize := int64(dec.PCMSize)
	if pcmSize == 0 {
		currentPos, _ := r.Seek(0, io.SeekCurrent)
		endPos, _ := r.Seek(0, io.SeekEnd)
		fileSize := endPos
		r.Seek(currentPos, io.SeekStart)

		if fileSize > 44 {
			pcmSize = fileSize - currentPos
			if pcmSize <= 0 {
				pcmSize = fileSize - 44
			}
		}
	}

	numChans := int64(dec.NumChans)
	bitDepth := int64(dec.BitDepth)
	sampleRate := float64(dec.SampleRate)
	if sampleRate == 0 || numChans == 0 || bitDepth == 0 {
		return 0, errors.New("invalid wav header metadata")
	}

	bytesPerFrame := numChans * (bitDepth / 8)
	if bytesPerFrame == 0 {
		return 0, errors.New("invalid byte depth calculation")
	}

	totalFrames := pcmSize / bytesPerFrame
	return float64(totalFrames) / sampleRate, nil
}

func getFLACDuration(r io.Reader) (float64, error) {
	stream, err := flac.Parse(r)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse flac stream")
	}
	defer stream.Close()

	return float64(stream.Info.NSamples) / float64(stream.Info.SampleRate), nil
}

func getM4ADuration(r io.ReadSeeker) (float64, error) {
	info, err := mp4.Probe(r)
	if err != nil {
		return 0, errors.Wrap(err, "failed to probe m4a/mp4 file")
	}
	return float64(info.Duration) / float64(info.Timescale), nil
}

func getOGGDuration(r io.ReadSeeker) (float64, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, errors.Wrap(err, "failed to seek ogg file")
	}

	reader, err := oggvorbis.NewReader(r)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create ogg vorbis reader")
	}

	channels := reader.Channels()
	sampleRate := reader.SampleRate()
	var totalSamples int64
	buf := make([]float32, 4096*channels)
	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, errors.Wrap(err, "failed to read ogg samples")
		}
		totalSamples += int64(n / channels)
	}

	return float64(totalSamples) / float64(sampleRate), nil
}

func getOpusDuration(r io.ReadSeeker) (float64, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, errors.Wrap(err, "failed to seek opus file")
	}

	var totalGranulePos int64
	buf := make([]byte, 27)
	for {
		n, err := r.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, errors.Wrap(err, "failed to read opus/ogg page")
		}
		if n < 27 {
			break
		}

		if string(buf[0:4]) != "OggS" {
			if _, err := r.Seek(-26, io.SeekCurrent); err != nil {
				break
			}
			continue
		}

		granulePos := int64(binary.LittleEndian.Uint64(buf[6:14]))
		if granulePos > totalGranulePos {
			totalGranulePos = granulePos
		}

		numSegments := int(buf[26])
		segmentTable := make([]byte, numSegments)
		if _, err := io.ReadFull(r, segmentTable); err != nil {
			break
		}

		var pageSize int
		for _, segSize := range segmentTable {
			pageSize += int(segSize)
		}
		if _, err := r.Seek(int64(pageSize), io.SeekCurrent); err != nil {
			break
		}
	}

	return float64(totalGranulePos) / 48000.0, nil
}

func getAIFFDuration(r io.ReadSeeker) (float64, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, errors.Wrap(err, "failed to seek aiff file")
	}

	dec := aiff.NewDecoder(r)
	if !dec.IsValidFile() {
		return 0, errors.New("invalid aiff file")
	}

	d, err := dec.Duration()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get aiff duration")
	}

	return d.Seconds(), nil
}

func getWebMDuration(r io.ReadSeeker) (float64, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, errors.Wrap(err, "failed to seek webm file")
	}

	buf := make([]byte, 8192)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return 0, errors.Wrap(err, "failed to read webm file")
	}

	if n > 0 && len(buf) >= 4 && binary.BigEndian.Uint32(buf[0:4]) == 0x1A45DFA3 {
		return 0, errors.New("webm duration parsing requires full EBML parser (consider using ffprobe for webm files)")
	}
	return 0, errors.New("failed to parse webm file")
}

func getAACDuration(r io.ReadSeeker) (float64, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, errors.Wrap(err, "failed to seek aac file")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read aac file")
	}

	var totalFrames int64
	var sampleRate int
	codec.SplitAACFrame(data, func(aac []byte) {
		if len(aac) >= 7 {
			asc, err := codec.ConvertADTSToASC(aac)
			if err == nil && sampleRate == 0 {
				sampleRate = codec.AACSampleIdxToSample(int(asc.Sample_freq_index))
			}
			totalFrames++
		}
	})

	if sampleRate == 0 || totalFrames == 0 {
		return 0, errors.New("no valid aac frames found")
	}

	totalSamples := totalFrames * 1024
	return float64(totalSamples) / float64(sampleRate), nil
}
