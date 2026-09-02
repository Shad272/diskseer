//go:build windows

package elevate

import "testing"

// Elevato deve rispondere senza andare in errore anche quando i privilegi non
// ci sono: è il caso normale, non un'eccezione.
func TestElevatoRispondeSempre(t *testing.T) {
	t.Logf("privilegi di amministratore: %v", Elevato())
}

// Gli argomenti con spazi vanno racchiusi fra virgolette, altrimenti il
// processo elevato riceverebbe --cliente "Mario Rossi" spezzato in due e il
// referto uscirebbe intestato a "Mario".
func TestComponiArgomentiProteggeGliSpazi(t *testing.T) {
	casi := []struct {
		in   []string
		vuoi string
	}{
		{nil, ""},
		{[]string{"--json"}, "--json"},
		{[]string{"--cliente", "Mario Rossi"}, `--cliente "Mario Rossi"`},
		{[]string{"--html", `C:\Documenti miei\r.html`}, `--html "C:\Documenti miei\r.html"`},
		{[]string{"--tecnico", `Anna "detta Ann"`}, `--tecnico "Anna \"detta Ann\""`},
	}
	for _, c := range casi {
		if got := componiArgomenti(c.in); got != c.vuoi {
			t.Errorf("componiArgomenti(%q) = %q, atteso %q", c.in, got, c.vuoi)
		}
	}
}
