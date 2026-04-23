package port

type IDGenerator interface {
	Generate() string
}
