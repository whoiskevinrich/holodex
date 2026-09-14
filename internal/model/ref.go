package model

import (
	"encoding/json"
	"strconv"
)

// Kind names one of the five entity kinds that carry a reference handle
// (F60 RD1, ADR-096 D1). The string is the `kind` half of `kind:id`.
type Kind string

const (
	KindPerson Kind = "person"
	KindStudio Kind = "studio"
	KindTag    Kind = "tag"
	KindFilm   Kind = "film"
	KindVideo  Kind = "video"
)

// Ref formats the reference handle `kind:id` (`film:42`). The server is the only
// producer — clients copy the string from a payload's `ref` and never assemble it.
func Ref(kind Kind, id int64) string {
	return string(kind) + ":" + strconv.FormatInt(id, 10)
}

// Each entity emits `ref` alongside its fields on every JSON encode, so list
// items, detail bodies and nested payloads (Video.People, Film cast, search
// results) all carry it without each writer remembering to. The alias type
// strips the method so the inner encode does not recurse.

// MarshalJSON adds `ref` to the encoded Video.
func (v Video) MarshalJSON() ([]byte, error) {
	type alias Video
	return json.Marshal(struct {
		alias
		Ref string `json:"ref"`
	}{alias(v), Ref(KindVideo, v.ID)})
}

// MarshalJSON adds `ref` to the encoded Person.
func (p Person) MarshalJSON() ([]byte, error) {
	type alias Person
	return json.Marshal(struct {
		alias
		Ref string `json:"ref"`
	}{alias(p), Ref(KindPerson, p.ID)})
}

// MarshalJSON adds `ref` to the encoded Studio.
func (s Studio) MarshalJSON() ([]byte, error) {
	type alias Studio
	return json.Marshal(struct {
		alias
		Ref string `json:"ref"`
	}{alias(s), Ref(KindStudio, s.ID)})
}

// MarshalJSON adds `ref` to the encoded Tag.
func (t Tag) MarshalJSON() ([]byte, error) {
	type alias Tag
	return json.Marshal(struct {
		alias
		Ref string `json:"ref"`
	}{alias(t), Ref(KindTag, t.ID)})
}

// MarshalJSON adds `ref` to the encoded Film.
func (f Film) MarshalJSON() ([]byte, error) {
	type alias Film
	return json.Marshal(struct {
		alias
		Ref string `json:"ref"`
	}{alias(f), Ref(KindFilm, f.ID)})
}
