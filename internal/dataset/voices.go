package dataset

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// Voices for text to speech: Piper voice models (VITS, exported to ONNX) from the
// rhasspy/piper-voices collection, pinned to one revision and checked against the
// SHA-256 of each model. They are about 750 MB together, so a server downloads them; they
// are never in the repository or a release. Every voice keeps the licence of the
// recordings it was trained on, which the API names with each answer.

func init() {
	register(&Spec{
		Name:   "voices",
		Schema: 1,
		Build:  buildVoices,
		Import: importVoices,
		// The voices are pinned: the check costs nothing until the pinned set changes.
		Every: 30 * 24 * time.Hour,
		Main:  "voices", MaxShrink: 0.5,
		NoSeed: true,
	})
}

// Voice is one Piper voice the API offers.
type Voice struct {
	ID       string // Piper's own name, such as hi_IN-rohan-medium
	Language string // BCP 47
	Name     string // the language, in English
	Speaker  string
	path     string // folder in the collection
	sha256   string
	// Licence of the recordings the voice was trained on, and the credit it asks for.
	Licence     string
	LicenceURL  string
	Attribution string
	// NonCommercial marks a voice whose licence forbids commercial use of what it makes.
	NonCommercial bool
}

const (
	piperVoicesRevision = "c10ece1aade47bb51c153c893d14e5bf8e5b7117"
	// voicesVersion changes whenever the list below does.
	voicesVersion = "piper-voices@c10ece1a/10"

	iitmNotice  = "Trained on the Indic TTS database of IIT Madras. COPYRIGHT 2016 TTS Consortium, TDIL, Meity represented by Hema A Murthy & S Umesh, DEPARTMENT OF Computer Science and Engineering and Electrical Engineering, IIT Madras. ALL RIGHTS RESERVED"
	iitmLicence = "IIT Madras Indic TTS licence: royalty-free, may be redistributed, notice kept"
	iitmURL     = "https://www.iitm.ac.in/donlab/indictts/downloads/license.pdf"
)

// Voices lists every voice, in the order the API shows them. The Marathi voice in the
// collection (mr_IN-google-medium) is left out: it was trained with a newer phoneme set
// than the 2023 Piper release reads, and Piper stops on it.
var Voices = []Voice{
	{ID: "en_US-ljspeech-medium", Language: "en-US", Name: "English (US)", Speaker: "LJ", path: "en/en_US/ljspeech/medium",
		sha256:  "6f52a751e2349abe7a76735eb09dc1875298c77ea2342ffd2fef79ff81b87f22",
		Licence: "Public domain (LJ Speech)", LicenceURL: "https://keithito.com/LJ-Speech-Dataset/", Attribution: "Trained on the LJ Speech dataset by Keith Ito, in the public domain."},
	{ID: "en_GB-cori-medium", Language: "en-GB", Name: "English (UK)", Speaker: "Cori", path: "en/en_GB/cori/medium",
		sha256:  "1899f98e5fb8310154f3c2973f4b8a929ba7245e722b3d3a85680b833d95f10d",
		Licence: "Public domain (LibriVox)", LicenceURL: "https://librivox.org/pages/public-domain/", Attribution: "Trained on public-domain LibriVox recordings read by Cori Samuel."},
	{ID: "hi_IN-rohan-medium", Language: "hi-IN", Name: "Hindi", Speaker: "Rohan", path: "hi/hi_IN/rohan/medium",
		sha256:  "b65dc80fb34d9dcd1cf684cb297966a34983bbc93bb1696fe207f32b0b33a091",
		Licence: iitmLicence, LicenceURL: iitmURL, Attribution: iitmNotice},
	{ID: "hi_IN-pratham-medium", Language: "hi-IN", Name: "Hindi", Speaker: "Pratham", path: "hi/hi_IN/pratham/medium",
		sha256:  "169964b0871667f6793416d4b35e97357a68ba1ad01df8580c28048989ee7693",
		Licence: "CC BY-NC-SA 4.0", LicenceURL: "https://creativecommons.org/licenses/by-nc-sa/4.0/", NonCommercial: true,
		Attribution: "Trained on AI4Bharat data (CC BY-NC-SA 4.0) by PravalX. Non-commercial use only."},
	{ID: "hi_IN-priyamvada-medium", Language: "hi-IN", Name: "Hindi", Speaker: "Priyamvada", path: "hi/hi_IN/priyamvada/medium",
		sha256:  "aa63bcf2cd493b55a450f280e23cf77f03afc9af7015e6e5acd43b652f166c88",
		Licence: "CC BY-NC-SA 4.0", LicenceURL: "https://creativecommons.org/licenses/by-nc-sa/4.0/", NonCommercial: true,
		Attribution: "Trained on AI4Bharat data (CC BY-NC-SA 4.0) by PravalX. Non-commercial use only."},
	{ID: "te_IN-padmavathi-medium", Language: "te-IN", Name: "Telugu", Speaker: "Padmavathi", path: "te/te_IN/padmavathi/medium",
		sha256:  "414aa5960d91ceb6e45bbdf8c27fdc71af09f205130d7be4e99470f3c2cfa57d",
		Licence: "CC BY 4.0", LicenceURL: "https://creativecommons.org/licenses/by/4.0/", Attribution: "Trained on AI4Bharat IndicVoices-R (CC BY 4.0) by PravalX."},
	{ID: "te_IN-venkatesh-medium", Language: "te-IN", Name: "Telugu", Speaker: "Venkatesh", path: "te/te_IN/venkatesh/medium",
		sha256:  "dfaa5b7833cd48d946f3fe18c9c934aaa4e8590aac6922fddf34783a694c3c87",
		Licence: "CC BY 4.0", LicenceURL: "https://creativecommons.org/licenses/by/4.0/", Attribution: "Trained on AI4Bharat IndicVoices-R (CC BY 4.0) by PravalX."},
	{ID: "te_IN-maya-medium", Language: "te-IN", Name: "Telugu", Speaker: "Maya", path: "te/te_IN/maya/medium",
		sha256:  "c3518ad4e3ca8ea6059c1e002f3772068f634960f58b237a96ff629db1c6200e",
		Licence: iitmLicence, LicenceURL: iitmURL, Attribution: iitmNotice},
	{ID: "ml_IN-arjun-medium", Language: "ml-IN", Name: "Malayalam", Speaker: "Arjun", path: "ml/ml_IN/arjun/medium",
		sha256:  "e881130516a874306972a07dcf262e6900140430c5658131121744a80ef3f11b",
		Licence: iitmLicence, LicenceURL: iitmURL, Attribution: iitmNotice},
	{ID: "ml_IN-meera-medium", Language: "ml-IN", Name: "Malayalam", Speaker: "Meera", path: "ml/ml_IN/meera/medium",
		sha256:  "0c3e730f8294286694cac5d33f4c94d050ed8ea74c5fd6d0d492d38cb57b5102",
		Licence: iitmLicence, LicenceURL: iitmURL, Attribution: iitmNotice},
}

// VoiceFiles names the attachments holding a voice's model and its settings.
func VoiceFiles(id string) (model, config string) { return id + "-model", id + "-config" }

func buildVoices(b *Builder) error {
	for _, voice := range Voices {
		model, config := VoiceFiles(voice.ID)
		if err := b.Attach(b.Path("voices", voice.ID+".onnx"), model); err != nil {
			return fmt.Errorf("%s: %w", voice.ID, err)
		}
		if err := b.Attach(b.Path("voices", voice.ID+".onnx.json"), config); err != nil {
			return fmt.Errorf("%s: %w", voice.ID, err)
		}
	}
	b.Count("voices", len(Voices))
	return nil
}

func importVoices(im *Importer) (Source, error) {
	if im.CurrentVersion() == voicesVersion {
		return Source{}, ErrNotChanged
	}
	base := "https://huggingface.co/rhasspy/piper-voices/resolve/" + piperVoicesRevision + "/"
	for _, voice := range Voices {
		model, err := im.Download(base+voice.path+"/"+voice.ID+".onnx", "voices/"+voice.ID+".onnx")
		if err != nil {
			return Source{}, err
		}
		if err := checkDigest(model, voice.sha256); err != nil {
			return Source{}, fmt.Errorf("%s: %w", voice.ID, err)
		}
		if _, err := im.Download(base+voice.path+"/"+voice.ID+".onnx.json", "voices/"+voice.ID+".onnx.json"); err != nil {
			return Source{}, err
		}
	}
	return Source{
		Name:        "Piper voices",
		Publisher:   "Rhasspy (Open Home Foundation)",
		URL:         "https://huggingface.co/rhasspy/piper-voices",
		Licence:     "Each voice keeps its training data's licence, named with every answer",
		LicenceURL:  "https://huggingface.co/rhasspy/piper-voices",
		Version:     voicesVersion,
		Attribution: "Speech by Piper (MIT) with voices from rhasspy/piper-voices.",
	}, nil
}

// checkDigest compares a downloaded file with the checksum it was pinned to.
func checkDigest(path, want string) error {
	got, err := fileDigest(path)
	if err != nil {
		return err
	}
	if got != want {
		os.Remove(path)
		return errors.New("checksum does not match the pinned file: " + got)
	}
	return nil
}
