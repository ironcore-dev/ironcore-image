package sets

type Empty struct{}

type String map[string]Empty

func (s String) Insert(items ...string) {
	for _, item := range items {
		s[item] = Empty{}
	}
}

func (s String) Has(item string) bool {
	_, ok := s[item]
	return ok
}

func (s String) Delete(items ...string) {
	for _, item := range items {
		delete(s, item)
	}
}

func NewString(items ...string) String {
	s := make(String)
	s.Insert(items...)
	return s
}
