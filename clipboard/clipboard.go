package clipboard

import (
	"bufio"
	"encoding/base64"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/mattn/go-gtk/gdkpixbuf"
)

// ClipboardType shows the type of the clipboard that is operated
type ClipboardType int

const (
	TEXT = iota
	IMAGE
)

func (t ClipboardType) String() string {
	return [...]string{"Text", "Image"}[t]
}

func GetEncodedClipboard() (encodedClipboard string, clipType ClipboardType, err error) {
	gtk.Init(nil)
	clip, err := gtk.ClipboardGet(gdk.SELECTION_CLIPBOARD)
	if err != nil {
		log.Fatalln(err)
		return "", 0, err
	}

	// Check if text clipboad is available
	if clip.WaitIsTextAvailable() {
		oldClipContent, err := clip.WaitForText()
		if err != nil {
			log.Fatalln(err)
			return "", 0, err
		}

		encodedClip := Encode([]byte(oldClipContent))
		if err != nil {
			log.Fatalln(err)
			return "", 0, err
		}
		return encodedClip, TEXT, nil
	}

	// Check if image clipboard is available
	if clip.WaitIsImageAvailable() {
		oldClipContent, err := clip.WaitForImage()
		if err != nil {
			log.Fatalln(err)
			return "", 0, err
		}
		tempFilePath := "temp.png"
		oldClipContent.SavePNG(tempFilePath, 0)
		encodedClip := base64EncodeImageFile(tempFilePath)
		os.Remove(tempFilePath)
		return encodedClip, IMAGE, nil
	}
	return "", 0, fmt.Errorf("Failed to get clipboard image/text")

}

func base64EncodeImageFile(filePath string) string {
	//  Open file on disk.
	f, _ := os.Open(filePath)
	defer f.Close()

	// Read entire JPG into byte slice.
	reader := bufio.NewReader(f)
	content, _ := ioutil.ReadAll(reader)

	// Encode as base64.
	encoded := base64.StdEncoding.EncodeToString(content)

	// Print encoded data to console.
	// fmt.Println("ENCODED: " + encoded)
	return encoded
}

func StoreClipboard(content string, clipType ClipboardType) error {
	gtk.Init(nil)
	clip, err := gtk.ClipboardGet(gdk.SELECTION_CLIPBOARD)
	if err != nil {
		log.Fatalln(err)
		return err
	}

	switch clipType {
	case TEXT:
		return storeTextClipBoard(clip, content)
	case IMAGE:
		return storeImageClipboard(clip, content)
	default:
		return errors.New("Type not implemented: " + clipType.String())
	}
}

func storeTextClipBoard(clip *gtk.Clipboard, content string) error {
	decodedContentBytes, err := Decode(content)
	if err != nil {
		return fmt.Errorf("Error decoding content: %s", content)
	}
	decodedContent := string(decodedContentBytes)

	// some clipboard managers allow to 'delete' entries while they seem to
	// be still there. inserting a new line forces them to redraw on top again
	if clip.WaitIsTextAvailable() {
		oldClipContent, err := clip.WaitForText()
		if err != nil {
			log.Fatalln(err)
			return err
		}
		if oldClipContent == decodedContent {
			clip.SetText("\n")
		} else {
			log.Println(fmt.Print("Setting clipboard content:", decodedContent))
			clip.SetText(decodedContent)
		}
	} else {
		log.Println("Setting clipboard content:", decodedContent)
		clip.SetText(decodedContent)
	}
	go func() { time.Sleep(100 * time.Millisecond); gtk.MainQuit() }()
	gtk.Main()
	return nil
}

func storeImageClipboard(clip *gtk.Clipboard, content string) error {
	decodedImage := make([]byte, len(content))
	b64.StdEncoding.Decode(decodedImage, []byte(content))

	tempPixbufFile := "temp.png"
	tempPixbuf, _ := gdkpixbuf.NewPixbufFromBytes(decodedImage)
	tempPixbuf.Save(tempPixbufFile, "png")
	decodedPixbuf, _ := gdk.PixbufNewFromFile(tempPixbufFile)
	os.Remove(tempPixbufFile)

	clip.SetImage(decodedPixbuf)
	clip.Store()
	go func() { time.Sleep(100 * time.Millisecond); gtk.MainQuit() }()
	gtk.Main()
	return nil
}

// Base64 encode content
func Encode(content []byte) string {
	return b64.StdEncoding.EncodeToString(content)
}

func Decode(content string) ([]byte, error) {
	return b64.StdEncoding.DecodeString(content)
}
