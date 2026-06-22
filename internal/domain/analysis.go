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

var sources = []string{"лаб", "цех", "посылка"}

func Sources() []string {
	out := make([]string, len(sources))
	copy(out, sources)
	return out
}

func ValidSource(s string) bool {
	if s == "" {
		return true
	}
	for _, x := range sources {
		if x == s {
			return true
		}
	}
	return false
}

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

var characteristicOptions = []string{
	"ηдин, спз",
	"ηдин, мПа•с",
	"[η], дл/г",
	"W, г/г",
	"м.д.н.в., %",
	"ρ20 г/см3",
	"ρ25, г/см3",
	"с.г., %",
	"pH",
	"АК, ppm",
	"АА, ppm",
}

func CharacteristicOptions() []string {
	out := make([]string, len(characteristicOptions))
	copy(out, characteristicOptions)
	return out
}

func ValidCharacteristicOption(name string) bool {
	for _, x := range characteristicOptions {
		if x == name {
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

type Characteristic struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Analysis struct {
	SchemaVersion   int              `json:"schema_version"`
	ID              string           `json:"id"`
	AnalysisDate    string           `json:"analysis_date"`
	SynthesisDate   string           `json:"synthesis_date"`
	Product         string           `json:"product"`
	Origin          string           `json:"origin"`
	Source          string           `json:"source"`
	Batch           string           `json:"batch"`
	Operator        string           `json:"operator"`
	SampleName      string           `json:"sample_name"`
	Description     string           `json:"description"`
	ShortResult     string           `json:"short_result"`
	Status          string           `json:"status"`
	Comment         string           `json:"comment"`
	Characteristics []Characteristic `json:"characteristics"`
	Attachments     Attachments      `json:"attachments"`
	CreatedAt       string           `json:"created_at"`
	UpdatedAt       string           `json:"updated_at"`
	Committed       bool             `json:"committed"`
	Deleted         bool             `json:"deleted"`
}

func (a *Analysis) Clone() *Analysis {
	c := *a
	c.Characteristics = append([]Characteristic(nil), a.Characteristics...)
	c.Attachments.Photos = append([]string(nil), a.Attachments.Photos...)
	c.Attachments.Spectra = append([]string(nil), a.Attachments.Spectra...)
	return &c
}
