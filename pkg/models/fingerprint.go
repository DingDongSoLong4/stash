package models

var (
	FingerprintTypeOshash = "oshash"
	FingerprintTypeMD5    = "md5"
	FingerprintTypePhash  = "phash"
)

// Fingerprint represents a fingerprint of a file.
type Fingerprint struct {
	Type        string
	Fingerprint string
}

func (f *Fingerprint) Value() string {
	return f.Fingerprint
}

type Fingerprints []Fingerprint

func (f *Fingerprints) Remove(type_ string) {
	var ret Fingerprints

	for _, ff := range *f {
		if ff.Type != type_ {
			ret = append(ret, ff)
		}
	}

	*f = ret
}

// Equals returns true if the contents of this slice are equal to those in the other slice.
func (f Fingerprints) Equals(other Fingerprints) bool {
	if len(f) != len(other) {
		return false
	}

	return !f.ContentsChanged(other)
}

// ContentsChanged returns true if this Fingerprints slice contains any Fingerprints that different Fingerprint values for the matching type in other, or if this slice contains any Fingerprint types that are not in other.
func (f Fingerprints) ContentsChanged(other Fingerprints) bool {
	for _, ff := range f {
		oo := other.For(ff.Type)
		if oo == nil || oo.Fingerprint != ff.Fingerprint {
			return true
		}
	}

	return false
}

// For returns a pointer to the first Fingerprint element matching the provided type.
func (f Fingerprints) For(type_ string) *Fingerprint {
	for _, fp := range f {
		if fp.Type == type_ {
			return &fp
		}
	}

	return nil
}

func (f Fingerprints) Get(type_ string) string {
	for _, fp := range f {
		if fp.Type == type_ {
			return fp.Fingerprint
		}
	}

	return ""
}

// AppendUnique appends a fingerprint to the list if a Fingerprint of the same type does not already exist in the list. If one does, then it is updated with o's Fingerprint value.
func (f Fingerprints) AppendUnique(o Fingerprint) Fingerprints {
	ret := f
	for i, fp := range ret {
		if fp.Type == o.Type {
			ret[i] = o
			return ret
		}
	}

	return append(f, o)
}
