package validate

import "testing"

func TestAmountCents(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
		ok   bool
	}{
		{10, 1000, true},
		{19.99, 1999, true},
		{0.1 + 0.2, 30, true}, // ruído de ponto flutuante é aceito
		{10.005, 0, false},    // 3 casas
		{0, 0, false},
		{-5, 0, false},
		{51, 0, false}, // acima do teto de 50
	}
	for _, c := range cases {
		got, err := AmountCents(c.in, 50)
		if (err == nil) != c.ok || (c.ok && got != c.want) {
			t.Errorf("AmountCents(%v) = %d, %v", c.in, got, err)
		}
	}
}

func TestPixKey(t *testing.T) {
	cases := []struct {
		kind, key string
		ok        bool
	}{
		{"CPF", "111.444.777-35", true},
		{"cpf", "11144477735", true},
		{"CPF", "1114447773", false},
		{"EMAIL", "a@b.com", true},
		{"EMAIL", "sem-arroba", false},
		{"PHONE", "+5511999998888", true},
		{"PHONE", "123", false},
		{"XYZ", "abc", false},
		{"CPF", "", false},
	}
	for _, c := range cases {
		if _, _, err := PixKey(c.kind, c.key); (err == nil) != c.ok {
			t.Errorf("PixKey(%q, %q) err=%v, esperado ok=%v", c.kind, c.key, err, c.ok)
		}
	}
}

func TestReference(t *testing.T) {
	for ref, ok := range map[string]bool{
		"agendamento-123": true, "a_b-C": true, "": false,
		"tem espaço": false, "com@arroba": false,
	} {
		if err := Reference(ref); (err == nil) != ok {
			t.Errorf("Reference(%q) err=%v", ref, err)
		}
	}
}