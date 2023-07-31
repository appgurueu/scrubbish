/*
Scrubbish takes metadata (EXIF, copyright info, comments) from a source JPEG file
and replaces (or strips, if no source is provided) the metadata of a destination JPEG file with it.

Usage:

    scrubbish [flags] [source] destination

The only flag is:

    -strip-trailer
        Strip trailing data after EOI.
        By default, trailing data (in either source or destination) will raise an error.

The source is optional; if none is provided, destination will be stripped of metadata,
otherwise, the metadata of the destination will be replaced with that of the source.

The destination is backed up to destination~ during the operation.
After the operation succeeds, the backup is removed.
*/
package main

import (
	"os"
	"io"
	"bufio"
	"errors"
	"fmt"
	"flag"
)

var stripTrailer = flag.Bool("strip-trailer", false, "Strip an eventual trailer")
func main() {
	flag.Parse()
	var from, to string
	switch flag.NArg() {
		case 1:
			to = flag.Arg(0)
		case 2:
			from, to = flag.Arg(0), flag.Arg(1)
		default:
			fmt.Println("usage: scrubbish [flags] [source] destination")
			return
	}
	err := replaceMetadata(to, from)
	if err != nil {
		fmt.Println("scrubbish:", err)
	}
}

// Replaces the metadata of toPath with that of fromPath (may be empty for stripping),
// creating a temporary copy of toPath at toPath~ in the process.
func replaceMetadata(toPath, fromPath string) error {
	copyPath := toPath + "~"
	err := os.Rename(toPath, copyPath)
	if err != nil { return err }
    defer os.Remove(copyPath)
    return merge(toPath, copyPath, fromPath)
}

// Reads the metadata from metadataImagePath
// (which may be empty, in which case the metadata is tripped)
// and everything else from imagePath, writing the result to outImagePath.
func merge(outImagePath, imagePath, metadataImagePath string) error {
    outFile, err := os.Create(outImagePath)
    if err != nil { return err }
    defer outFile.Close()
    writer := bufio.NewWriter(outFile)

    imageFile, err := os.Open(imagePath)
    if err != nil { return err }
    defer imageFile.Close()
    imageReader := bufio.NewReader(imageFile)

	_, err = writer.Write([]byte{0xFF, soi})
	if err != nil { return err }
	{
		if metadataImagePath != "" {
			// Copy metadata segments
			// It seems that they need to come first!
			metaFile, err := os.Open(metadataImagePath)
	   		if err != nil { return err }
	    	defer metaFile.Close()
	    	metaReader := bufio.NewReader(metaFile)
			err = copySegments(writer, metaReader, isMetaTagType)
			if err != nil { return err }
		}
		// Copy all non-metadata segments
		err = copySegments(writer, imageReader, func(tagType byte) bool {
			return !isMetaTagType(tagType)
		})
		if err != nil { return err }
	}
	_, err = writer.Write([]byte{0xFF, eoi})
	if err != nil { return err }

    // Flush the writer, otherwise the last couple buffered writes (including the EOI) won't get written!
    return writer.Flush()
}

// This does not decode JPEGs; it only parses and understands them at a segment level.

const (
	soi = 0xD8
	eoi = 0xD9
	sos = 0xDA
	app1 = 0xE1 // typically EXIF
	app14 = 0xEE // typically copyright info
	com = 0xFE
)

func isMetaTagType(tagType byte) bool {
	return (tagType >= app1 && tagType <= app14) || tagType == com
}

func copySegments(dst *bufio.Writer, src *bufio.Reader, filterSegment func(tagType byte) bool) error {
	var buf [2]byte
	_, err := io.ReadFull(src, buf[:])
	if err != nil { return err }
	if buf != [2]byte{0xFF, soi} {
		return errors.New("expected SOI")
	}
	for {
		_, err := io.ReadFull(src, buf[:])
		if err != nil { return err }
		if buf[0] != 0xFF {
			return errors.New("invalid tag type")
		}
		if buf[1] == eoi {
			if !*stripTrailer {
				// Hacky way to check for EOF
				n, err := src.Read(buf[:1])
				if err != nil && err != io.EOF { return err }
				if n > 0 {
					return errors.New("unexpected trailer")
				}
			}
			return nil
		}
		sos := buf[1] == 0xDA
		filter := filterSegment(buf[1])
		if filter {
			_, err = dst.Write(buf[:])
			if err != nil { return err }
		}

		_, err = io.ReadFull(src, buf[:])
		if err != nil { return err }
		if filter {
			_, err = dst.Write(buf[:])
			if err != nil { return err }
		}

		// Note: Includes the length, but not the tag, so subtract 2
		tagLength := ((uint16(buf[0]) << 8) | uint16(buf[1])) - 2
		if filter {
			_, err = io.CopyN(dst, src, int64(tagLength))
		} else {
			_, err = src.Discard(int(tagLength))
		}
		if err != nil { return err }
		if sos {
			// Find next tag `FF xx` (where `xx != 0` and `xx` isn't a restart marker) to skip ECS
			for {
				bytes, err := src.Peek(2)
				if err != nil { return err }
				if bytes[0] == 0xFF {
					data, rstMrk := bytes[1] == 0, bytes[1] >= 0xD0 && bytes[1] <= 0xD7
					if !data && !rstMrk {
						break
					}
				}
				if filter {
					err = dst.WriteByte(bytes[0])
					if err != nil { return err }
				}
				_, err = src.Discard(1)
				if err != nil { return err }
			}
		}
	}
}
