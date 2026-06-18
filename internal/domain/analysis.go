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

type Kind string

const (
	KindPhoto    Kind = "photo"
	KindSpectrum Kind = "spectrum"
	KindReport   Kind = "report"
)

func (k Kind) Folder() string {
	switch k {
	case KindPhoto:
		return "photos"
	case KindSpectrum:
		return "spectra"
	case KindReport:
		return "reports"
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
	case KindReport:
		return "report"
	default:
		return "file"
	}
}

func (k Kind) Valid() bool {
	return k == KindPhoto || k == KindSpectrum || k == KindReport
}

type Attachments struct {
	Photos  []string `json:"photos"`
	Spectra []string `json:"spectra"`
	Reports []string `json:"reports"`
}

func (a *Attachments) For(k Kind) *[]string {
	switch k {
	case KindPhoto:
		return &a.Photos
	case KindSpectrum:
		return &a.Spectra
	case KindReport:
		return &a.Reports
	default:
		return nil
	}
}

type Analysis struct {
	SchemaVersion int         `json:"schema_version"`
	ID            string      `json:"id"`
	AnalysisDate  string      `json:"analysis_date"`
	Product       string      `json:"product"`
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
}

func (a *Analysis) Clone() *Analysis {
	c := *a
	c.Attachments.Photos = append([]string(nil), a.Attachments.Photos...)
	c.Attachments.Spectra = append([]string(nil), a.Attachments.Spectra...)
	c.Attachments.Reports = append([]string(nil), a.Attachments.Reports...)
	return &c
}
