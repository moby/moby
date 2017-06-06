package iso9660wrap

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
)

const reservedAreaData string = `
Once upon a midnight dreary, while I pondered, weak and weary,
Over many a quaint and curious volume of forgotten lore—
    While I nodded, nearly napping, suddenly there came a tapping,
As of some one gently rapping, rapping at my chamber door.
“’Tis some visitor,” I muttered, “tapping at my chamber door—
            Only this and nothing more.”

 Ah, distinctly I remember it was in the bleak December;
And each separate dying ember wrought its ghost upon the floor.
    Eagerly I wished the morrow;—vainly I had sought to borrow
    From my books surcease of sorrow—sorrow for the lost Lenore—
For the rare and radiant maiden whom the angels name Lenore—
            Nameless here for evermore.

    And the silken, sad, uncertain rustling of each purple curtain
Thrilled me—filled me with fantastic terrors never felt before;
    So that now, to still the beating of my heart, I stood repeating
    “’Tis some visitor entreating entrance at my chamber door—
Some late visitor entreating entrance at my chamber door;—
            This it is and nothing more.”

    Presently my soul grew stronger; hesitating then no longer,
“Sir,” said I, “or Madam, truly your forgiveness I implore;
    But the fact is I was napping, and so gently you came rapping,
    And so faintly you came tapping, tapping at my chamber door,
That I scarce was sure I heard you”—here I opened wide the door;—
            Darkness there and nothing more.

    Deep into that darkness peering, long I stood there wondering, fearing,
Doubting, dreaming dreams no mortal ever dared to dream before;
    But the silence was unbroken, and the stillness gave no token,
    And the only word there spoken was the whispered word, “Lenore?”
This I whispered, and an echo murmured back the word, “Lenore!”—
            Merely this and nothing more.

    Back into the chamber turning, all my soul within me burning,
Soon again I heard a tapping somewhat louder than before.
    “Surely,” said I, “surely that is something at my window lattice;
      Let me see, then, what thereat is, and this mystery explore—
Let my heart be still a moment and this mystery explore;—
            ’Tis the wind and nothing more!”

    Open here I flung the shutter, when, with many a flirt and flutter,
In there stepped a stately Raven of the saintly days of yore;
    Not the least obeisance made he; not a minute stopped or stayed he;
    But, with mien of lord or lady, perched above my chamber door—
Perched upon a bust of Pallas just above my chamber door—
            Perched, and sat, and nothing more.

Then this ebony bird beguiling my sad fancy into smiling,
By the grave and stern decorum of the countenance it wore,
“Though thy crest be shorn and shaven, thou,” I said, “art sure no craven,
Ghastly grim and ancient Raven wandering from the Nightly shore—
Tell me what thy lordly name is on the Night’s Plutonian shore!”
            Quoth the Raven “Nevermore.”

    Much I marvelled this ungainly fowl to hear discourse so plainly,
Though its answer little meaning—little relevancy bore;
    For we cannot help agreeing that no living human being
    Ever yet was blessed with seeing bird above his chamber door—
Bird or beast upon the sculptured bust above his chamber door,
            With such name as “Nevermore.”

    But the Raven, sitting lonely on the placid bust, spoke only
That one word, as if his soul in that one word he did outpour.
    Nothing farther then he uttered—not a feather then he fluttered—
    Till I scarcely more than muttered “Other friends have flown before—
On the morrow he will leave me, as my Hopes have flown before.”
            Then the bird said “Nevermore.”

    Startled at the stillness broken by reply so aptly spoken,
“Doubtless,” said I, “what it utters is its only stock and store
    Caught from some unhappy master whom unmerciful Disaster
    Followed fast and followed faster till his songs one burden bore—
Till the dirges of his Hope that melancholy burden bore
            Of ‘Never—nevermore’.”

    But the Raven still beguiling all my fancy into smiling,
Straight I wheeled a cushioned seat in front of bird, and bust and door;
    Then, upon the velvet sinking, I betook myself to linking
    Fancy unto fancy, thinking what this ominous bird of yore—
What this grim, ungainly, ghastly, gaunt, and ominous bird of yore
            Meant in croaking “Nevermore.”

    This I sat engaged in guessing, but no syllable expressing
To the fowl whose fiery eyes now burned into my bosom’s core;
    This and more I sat divining, with my head at ease reclining
    On the cushion’s velvet lining that the lamp-light gloated o’er,
But whose velvet-violet lining with the lamp-light gloating o’er,
            She shall press, ah, nevermore!

    Then, methought, the air grew denser, perfumed from an unseen censer
Swung by Seraphim whose foot-falls tinkled on the tufted floor.
    “Wretch,” I cried, “thy God hath lent thee—by these angels he hath sent thee
    Respite—respite and nepenthe from thy memories of Lenore;
Quaff, oh quaff this kind nepenthe and forget this lost Lenore!”
            Quoth the Raven “Nevermore.”

    “Prophet!” said I, “thing of evil!—prophet still, if bird or devil!—
Whether Tempter sent, or whether tempest tossed thee here ashore,
    Desolate yet all undaunted, on this desert land enchanted—
    On this home by Horror haunted—tell me truly, I implore—
Is there—is there balm in Gilead?—tell me—tell me, I implore!”
            Quoth the Raven “Nevermore.”

    “Prophet!” said I, “thing of evil!—prophet still, if bird or devil!
By that Heaven that bends above us—by that God we both adore—
    Tell this soul with sorrow laden if, within the distant Aidenn,
    It shall clasp a sainted maiden whom the angels name Lenore—
Clasp a rare and radiant maiden whom the angels name Lenore.”
            Quoth the Raven “Nevermore.”

    “Be that word our sign of parting, bird or fiend!” I shrieked, upstarting—
“Get thee back into the tempest and the Night’s Plutonian shore!
    Leave no black plume as a token of that lie thy soul hath spoken!
    Leave my loneliness unbroken!—quit the bust above my door!
Take thy beak from out my heart, and take thy form from off my door!”
            Quoth the Raven “Nevermore.”

    And the Raven, never flitting, still is sitting, still is sitting
On the pallid bust of Pallas just above my chamber door;
    And his eyes have all the seeming of a demon’s that is dreaming,
    And the lamp-light o’er him streaming throws his shadow on the floor;
And my soul from out that shadow that lies floating on the floor
            Shall be lifted—nevermore!
`

func Panicf(format string, v ...interface{}) {
	panic(fmt.Errorf(format, v...))
}

const volumeDescriptorSetMagic = "\x43\x44\x30\x30\x31\x01"

const primaryVolumeSectorNum uint32 = 16
const numVolumeSectors uint32 = 2 // primary + terminator
const littleEndianPathTableSectorNum uint32 = primaryVolumeSectorNum + numVolumeSectors
const bigEndianPathTableSectorNum uint32 = littleEndianPathTableSectorNum + 1
const numPathTableSectors = 2 // no secondaries
const rootDirectorySectorNum uint32 = primaryVolumeSectorNum + numVolumeSectors + numPathTableSectors

// WriteFile writes the contents of infh to an iso at outfh with the name provided
func WriteFile(outfh, infh *os.File) error {
	fileSize, filename, err := getInputFileSizeAndName(infh)
	if err != nil {
		return err
	}
	if fileSize == 0 {
		return fmt.Errorf("input file must be at least 1 byte in size")
	}
	filename = strings.ToUpper(filename)
	if !filenameSatisfiesISOConstraints(filename) {
		return fmt.Errorf("Input file name %s does not satisfy the ISO9660 character set constraints", filename)
	}

	buf := make([]byte, fileSize, fileSize)
	_, err = infh.Read(buf)
	if err != nil {
		return err
	}

	return WriteBuffer(outfh, buf, filename)
}

// WriteBuffer writes the contents of buf to an iso at outfh with the name provided
func WriteBuffer(outfh *os.File, buf []byte, filename string) error {
	fileSize := uint32(len(buf))
	if fileSize == 0 {
		return fmt.Errorf("input buffer must be at least 1 byte in size")
	}
	r := bytes.NewReader(buf)

	// reserved sectors
	reservedAreaLength := int64(16 * SectorSize)
	_, err := outfh.Write([]byte(reservedAreaData))
	if err != nil {
		return fmt.Errorf("could not write to output file: %s", err)
	}
	err = outfh.Truncate(reservedAreaLength)
	if err != nil {
		return fmt.Errorf("could not truncate output file: %s", err)
	}
	_, err = outfh.Seek(reservedAreaLength, os.SEEK_SET)
	if err != nil {
		return fmt.Errorf("could not seek output file: %s", err)
	}

	err = nil
	func() {
		defer func() {
			var ok bool
			e := recover()
			if e != nil {
				err, ok = e.(error)
				if !ok {
					panic(e)
				}
			}
		}()

		bufw := bufio.NewWriter(outfh)

		w := NewISO9660Writer(bufw)

		writePrimaryVolumeDescriptor(w, fileSize, filename)
		writeVolumeDescriptorSetTerminator(w)
		writePathTable(w, binary.LittleEndian)
		writePathTable(w, binary.BigEndian)
		writeData(w, r, fileSize, filename)

		w.Finish()

		err := bufw.Flush()
		if err != nil {
			panic(err)
		}
	}()
	if err != nil {
		return fmt.Errorf("could not write to output file: %s", err)
	}
	return nil
}

func writePrimaryVolumeDescriptor(w *ISO9660Writer, fileSize uint32, filename string) {
	if len(filename) > 32 {
		filename = filename[:32]
	}
	now := time.Now()

	sw := w.NextSector()
	if w.CurrentSector() != primaryVolumeSectorNum {
		Panicf("internal error: unexpected primary volume sector %d", w.CurrentSector())
	}

	sw.WriteByte('\x01')
	sw.WriteString(volumeDescriptorSetMagic)
	sw.WriteByte('\x00')

	sw.WritePaddedString("", 32)
	sw.WritePaddedString(filename, 32)

	sw.WriteZeros(8)
	sw.WriteBothEndianDWord(numTotalSectors(fileSize))
	sw.WriteZeros(32)

	sw.WriteBothEndianWord(1) // volume set size
	sw.WriteBothEndianWord(1) // volume sequence number
	sw.WriteBothEndianWord(uint16(SectorSize))
	sw.WriteBothEndianDWord(SectorSize) // path table length

	sw.WriteLittleEndianDWord(littleEndianPathTableSectorNum)
	sw.WriteLittleEndianDWord(0) // no secondary path tables
	sw.WriteBigEndianDWord(bigEndianPathTableSectorNum)
	sw.WriteBigEndianDWord(0) // no secondary path tables

	WriteDirectoryRecord(sw, "\x00", rootDirectorySectorNum) // root directory

	sw.WritePaddedString("", 128) // volume set identifier
	sw.WritePaddedString("", 128) // publisher identifier
	sw.WritePaddedString("", 128) // data preparer identifier
	sw.WritePaddedString("", 128) // application identifier

	sw.WritePaddedString("", 37) // copyright file identifier
	sw.WritePaddedString("", 37) // abstract file identifier
	sw.WritePaddedString("", 37) // bibliographical file identifier

	sw.WriteDateTime(now)         // volume creation
	sw.WriteDateTime(now)         // most recent modification
	sw.WriteUnspecifiedDateTime() // expires
	sw.WriteUnspecifiedDateTime() // is effective (?)

	sw.WriteByte('\x01') // version
	sw.WriteByte('\x00') // reserved

	sw.PadWithZeros() // 512 (reserved for app) + 653 (zeros)
}

func writeVolumeDescriptorSetTerminator(w *ISO9660Writer) {
	sw := w.NextSector()
	if w.CurrentSector() != primaryVolumeSectorNum+1 {
		Panicf("internal error: unexpected volume descriptor set terminator sector %d", w.CurrentSector())
	}

	sw.WriteByte('\xFF')
	sw.WriteString(volumeDescriptorSetMagic)

	sw.PadWithZeros()
}

func writePathTable(w *ISO9660Writer, bo binary.ByteOrder) {
	sw := w.NextSector()
	sw.WriteByte(1) // name length
	sw.WriteByte(0) // number of sectors in extended attribute record
	sw.WriteDWord(bo, rootDirectorySectorNum)
	sw.WriteWord(bo, 1) // parent directory recno (root directory)
	sw.WriteByte(0)     // identifier (root directory)
	sw.WriteByte(1)     // padding
	sw.PadWithZeros()
}

func writeData(w *ISO9660Writer, infh io.Reader, fileSize uint32, filename string) {
	sw := w.NextSector()
	if w.CurrentSector() != rootDirectorySectorNum {
		Panicf("internal error: unexpected root directory sector %d", w.CurrentSector())
	}

	WriteDirectoryRecord(sw, "\x00", w.CurrentSector())
	WriteDirectoryRecord(sw, "\x01", rootDirectorySectorNum)
	WriteFileRecordHeader(sw, filename, w.CurrentSector()+1, fileSize)

	// Now stream the data.  Note that the first buffer is never of SectorSize,
	// since we've already filled a part of the sector.
	b := make([]byte, SectorSize)
	total := uint32(0)
	for {
		l, err := infh.Read(b)
		if err != nil && err != io.EOF {
			Panicf("could not read from input file: %s", err)
		}
		if l > 0 {
			sw = w.NextSector()
			sw.Write(b[:l])
			total += uint32(l)
		}
		if err == io.EOF {
			break
		}
	}
	if total != fileSize {
		Panicf("input file size changed while the ISO file was being created (expected to read %d, read %d)", fileSize, total)
	} else if w.CurrentSector() != numTotalSectors(fileSize)-1 {
		Panicf("internal error: unexpected last sector number (expected %d, actual %d)",
			numTotalSectors(fileSize)-1, w.CurrentSector())
	}
}

func numTotalSectors(fileSize uint32) uint32 {
	var numDataSectors uint32
	numDataSectors = (fileSize + (SectorSize - 1)) / SectorSize
	return 1 + rootDirectorySectorNum + numDataSectors
}

func getInputFileSizeAndName(fh *os.File) (uint32, string, error) {
	fi, err := fh.Stat()
	if err != nil {
		return 0, "", err
	}
	if fi.Size() >= math.MaxUint32 {
		return 0, "", fmt.Errorf("file size %d is too large", fi.Size())
	}
	return uint32(fi.Size()), fi.Name(), nil
}

func filenameSatisfiesISOConstraints(filename string) bool {
	invalidCharacter := func(r rune) bool {
		// According to ISO9660, only capital letters, digits, and underscores
		// are permitted.  Some sources say a dot is allowed as well.  I'm too
		// lazy to figure it out right now.
		if r >= 'A' && r <= 'Z' {
			return false
		} else if r >= '0' && r <= '9' {
			return false
		} else if r == '_' {
			return false
		} else if r == '.' {
			return false
		}
		return true
	}
	return strings.IndexFunc(filename, invalidCharacter) == -1
}
