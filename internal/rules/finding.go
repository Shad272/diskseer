package rules

import (
	"fmt"

	"github.com/shad272/diskseer/internal/i18n"
)

type Severity int

const (
	SevOK Severity = iota
	SevInfo
	SevWarn
	SevCritical
)

func (s Severity) Label(l i18n.Lingua) string {
	switch s {
	case SevCritical:
		return l.S("CRITICAL", "CRITICO")
	case SevWarn:
		return l.S("WARNING", "ATTENZIONE")
	case SevInfo:
		return l.S("INFO", "INFO")
	default:
		return l.S("OK", "OK")
	}
}

func (s Severity) String() string { return s.Label(i18n.EN) }

// Slug è la forma della severità utilizzabile come classe CSS.
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

// Finding è un verdetto, non una misura. Ogni campo ha un compito preciso:
//
//	Title    -> cosa c'è che non va, in una riga
//	Detail   -> perché lo sappiamo, con i numeri dentro
//	Action   -> cosa deve fare chi legge
//	Evidence -> i dati grezzi, per chi vuole verificare
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

	// SuiLimiti distingue i verdetti che parlano della diagnosi stessa da
	// quelli che parlano della macchina. Serve a ordinarli, e sta qui invece
	// di essere dedotto dall'area perché l'area è un'etichetta tradotta:
	// confrontare testo mostrato all'utente per prendere decisioni è il modo
	// più rapido di rompere un programma il giorno che si cambia una parola.
	SuiLimiti bool
}

func (f Finding) String() string {
	return fmt.Sprintf("[%s] %s - %s", f.Severity, f.Area, f.Title)
}

// builder raccoglie i verdetti e porta con sé la lingua, così ogni regola può
// scrivere le proprie frasi senza doversela passare in giro.
type builder struct {
	out []Finding
	l   i18n.Lingua
}

func (b *builder) add(f Finding) { b.out = append(b.out, f) }

// Scorciatoie perché le regole restino leggibili: b.s, b.f e b.n al posto di
// b.l.S, b.l.F e b.l.N.
func (b *builder) s(en, it string) string { return b.l.S(en, it) }

func (b *builder) f(en, it string, args ...any) string { return b.l.F(en, it, args...) }

func (b *builder) n(v uint64, enSing, enPlur, itSing, itPlur string) string {
	return b.l.N(v, enSing, enPlur, itSing, itPlur)
}
