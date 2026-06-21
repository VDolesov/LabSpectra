package storage

import (
	"path"
	"path/filepath"
)

type Paths struct {
	Root string
}

func NewPaths(root string) Paths {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return Paths{Root: abs}
}

func (p Paths) Registry() string { return filepath.Join(p.Root, "registry.xlsx") }
func (p Paths) Products() string { return filepath.Join(p.Root, "products.json") }
func (p Paths) Samples() string  { return filepath.Join(p.Root, "samples") }
func (p Paths) Backups() string  { return filepath.Join(p.Root, "backups") }
func (p Paths) Logs() string     { return filepath.Join(p.Root, "logs") }
func (p Paths) Trash() string    { return filepath.Join(p.Root, ".trash") }
func (p Paths) Lock() string     { return filepath.Join(p.Root, ".lock") }

func (p Paths) SampleDir(id string) string {
	return filepath.Join(p.Samples(), id)
}

func (p Paths) KindDir(id, folder string) string {
	return filepath.Join(p.SampleDir(id), folder)
}

func (p Paths) RelSampleDir(id string) string {
	return path.Join("samples", id)
}

func (p Paths) RelAttachment(id, rel string) string {
	return path.Join("samples", id, rel)
}

func (p Paths) AbsFromAnalysisRel(id, rel string) string {
	return filepath.Join(p.SampleDir(id), filepath.FromSlash(rel))
}
