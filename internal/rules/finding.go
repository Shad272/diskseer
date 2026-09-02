package rules

import "fmt"

type Severity int

const (
	SevOK Severity = iota
	SevInfo
	SevWarn
	SevCritical
)

func (s Severity) String() string {
	switch s {
	case SevCritical:
		return "CRITICO"
	case SevWarn:
		return "ATTENZIONE"
	case SevInfo:
		return "INFO"
	default:
		return "OK"
	}
}

// Finding è un verdetto, non una misura. Ogni campo ha un compito preciso:
//
//	Title   -> cosa c'è che non va, in una riga
//	Detail  -> perché lo sappiamo, con i numeri dentro
//	Action  -> cosa deve fare chi legge
//	Evidence-> i dati grezzi, per chi vuole verificare
//
// Un verdetto senza Action non serve a niente: è quello che già fanno tutti
// gli altri programmi di diagnostica.
type Finding struct {
	Severity Severity
	Area     string
	Target   string
	Title    string
	Detail   string
	Action   string
	Evidence map[string]string
}

func (f Finding) String() string {
	return fmt.Sprintf("[%s] %s - %s", f.Severity, f.Area, f.Title)
}

type builder struct{ out []Finding }

func (b *builder) add(f Finding) { b.out = append(b.out, f) }

// Slug e' la forma della severita' utilizzabile come classe CSS.
func (s Severity) Slug() string {
	switch s {
	case SevCritical:
		return "critical"
	case SevWarn:
		return "warn"
	case SevInfo:
		return "info"
	default:
		return "ok"
	}
}

// conta accorda il numero con il sostantivo che lo segue.
//
// Sembra una rifinitura e invece è sostanza: il referto finisce sotto gli
// occhi di un cliente, e "1 errori di trasmissione" fa sembrare approssimativo
// tutto quello che c'è scritto intorno, comprese le diagnosi giuste.
func conta(n uint64, singolare, plurale string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singolare)
	}
	return fmt.Sprintf("%d %s", n, plurale)
}
