package domain

const SchemaVersion = 1

type Status string

const (
	StatusNew    Status = "новый"
	StatusInWork Status = "в работе"
	StatusDone   Status = "готов"
)

func AllStatuses() []Status {
	return []Status{StatusNew, StatusInWork, StatusDone}
}

const OriginAcripol = "АКРИПОЛ"

var products = []string{"R2531", "V00S9", "PR4832", "E10K7", "E10H6", "КП 1020", "КП 540"}

func Products() []string {
	out := make([]string, len(products))
	copy(out, products)
	return out
}

func ValidProduct(p string) bool {
	if p == "" {
		return true
	}
	for _, x := range products {
		if x == p {
			return true
		}
	}
	return false
}

type Kind string

const (
	KindPhoto    Kind = "photo"
	KindSpectrum Kind = "spectrum"
)

func (k Kind) Folder() string {
	switch k {
	case KindPhoto:
		return "photos"
	case KindSpectrum:
		return "spectra"
	default:
		return ""
	}
}

func (k Kind) Prefix() string {
	switch k {
	case KindPhoto:
		return "photo"
	case KindSpectrum:
		return "spectrum"
	default:
		return "file"
	}
}

func (k Kind) Valid() bool {
	return k == KindPhoto || k == KindSpectrum
}

type Attachments struct {
	Photos  []string `json:"photos"`
	Spectra []string `json:"spectra"`
}

func (a *Attachments) For(k Kind) *[]string {
	switch k {
	case KindPhoto:
		return &a.Photos
	case KindSpectrum:
		return &a.Spectra
	default:
		return nil
	}
}

type Analysis struct {
	SchemaVersion int         `json:"schema_version"`
	ID            string      `json:"id"`
	AnalysisDate  string      `json:"analysis_date"`
	SynthesisDate string      `json:"synthesis_date"`
	Product       string      `json:"product"`
	Origin        string      `json:"origin"`
	Batch         string      `json:"batch"`
	SampleName    string      `json:"sample_name"`
	Description   string      `json:"description"`
	ShortResult   string      `json:"short_result"`
	Status        string      `json:"status"`
	Comment       string      `json:"comment"`
	Attachments   Attachments `json:"attachments"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	Committed     bool        `json:"committed"`
	Deleted       bool        `json:"deleted"`
}

func (a *Analysis) Clone() *Analysis {
	c := *a
	c.Attachments.Photos = append([]string(nil), a.Attachments.Photos...)
	c.Attachments.Spectra = append([]string(nil), a.Attachments.Spectra...)
	return &c
}
