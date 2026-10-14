package dataset

import "time"

// The background removal model: IS-Net "general use" from the DIS project by Xuebin Qin
// and others (Apache-2.0), in the ONNX form the rembg project publishes. 179 MB, pinned by
// checksum and fetched by a server; never in the repository or a release.

func init() {
	register(&Spec{
		Name:   "matting",
		Schema: 1,
		Build:  buildMatting,
		Import: importMatting,
		Every:  30 * 24 * time.Hour,
		Main:   "models", MaxShrink: 0,
		NoSeed: true,
	})
}

const (
	mattingURL     = "https://github.com/danielgatis/rembg/releases/download/v0.0.0/isnet-general-use.onnx"
	mattingSHA256  = "60920e99c45464f2ba57bee2ad08c919a52bbf852739e96947fbb4358c0d964a"
	mattingVersion = "isnet-general-use@rembg-v0.0.0"
	// MattingSize is the square the model looks at, in pixels.
	MattingSize = 1024
)

func buildMatting(b *Builder) error {
	if err := b.Attach(b.Path("isnet-general-use.onnx"), "isnet"); err != nil {
		return err
	}
	b.Count("models", 1)
	return nil
}

func importMatting(im *Importer) (Source, error) {
	if im.CurrentVersion() == mattingVersion {
		return Source{}, ErrNotChanged
	}
	path, err := im.Download(mattingURL, "isnet-general-use.onnx")
	if err != nil {
		return Source{}, err
	}
	if err := checkDigest(path, mattingSHA256); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "IS-Net (DIS), general-use weights",
		Publisher:   "Xuebin Qin et al.; ONNX export by the rembg project",
		URL:         "https://github.com/xuebinqin/DIS",
		Licence:     "Apache-2.0",
		LicenceURL:  "https://github.com/xuebinqin/DIS/blob/main/LICENSE.md",
		Version:     mattingVersion,
		SHA256:      mattingSHA256,
		Attribution: "Background removal with IS-Net (Qin et al., Highly Accurate Dichotomous Image Segmentation, ECCV 2022), Apache-2.0.",
	}, nil
}
