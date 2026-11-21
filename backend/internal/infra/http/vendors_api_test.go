package httpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// vendorFake guarda o que a API de fornecedores leu e gravou.
type vendorFake struct {
	plan     domain.VendorPlan
	vendors  []domain.Vendor
	created  []domain.Vendor
	initial  []*domain.VendorPayment
	updated  []domain.Vendor
	payments []domain.VendorPayment
	addErr   error
	found    bool
}

func (f *fakeReader) VendorPlan(context.Context) (domain.VendorPlan, error) {
	return f.vend.plan, f.err
}
func (f *fakeReader) SaveVendorPlan(_ context.Context, p domain.VendorPlan) error {
	f.vend.plan = p
	return f.err
}
func (f *fakeReader) Vendors(context.Context) ([]domain.Vendor, error) { return f.vend.vendors, f.err }
func (f *fakeReader) CreateVendor(_ context.Context, v domain.Vendor, initial *domain.VendorPayment) error {
	f.vend.created = append(f.vend.created, v)
	f.vend.initial = append(f.vend.initial, initial)
	return f.err
}
func (f *fakeReader) UpdateVendor(_ context.Context, v domain.Vendor) (bool, error) {
	f.vend.updated = append(f.vend.updated, v)
	return f.vend.found, f.err
}
func (f *fakeReader) DeleteVendor(context.Context, uuid.UUID) (bool, error) {
	return f.vend.found, f.err
}
func (f *fakeReader) AddVendorPayment(_ context.Context, p domain.VendorPayment) error {
	f.vend.payments = append(f.vend.payments, p)
	return f.vend.addErr
}
func (f *fakeReader) DeleteVendorPayment(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return f.vend.found, f.err
}

func vdate(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestAPI_Vendors(t *testing.T) {
	r := &fakeReader{
		accounts: []domain.Account{
			{Type: "BANK", Balance: 1000, Reserved: []domain.ReservedBalance{{Name: "Casamento", Amount: 1500}, {Name: "Reserva", Amount: 50}}},
			{Type: "BANK", Reserved: []domain.ReservedBalance{{Name: "Casamento", Amount: 500}}},
		},
		vend: vendorFake{
			plan: domain.VendorPlan{Name: "Casamento", ContributionDay: 10, SavingsBox: "Casamento"},
			vendors: []domain.Vendor{
				{ID: uuid.New(), Name: "Buffet", Contracted: 12000, DueDate: vdate(2027, time.August, 20),
					Payments: []domain.VendorPayment{{ID: uuid.New(), Amount: 2000, PaidOn: *vdate(2026, time.September, 1), Description: "PIX BUFFET"}}},
				{ID: uuid.New(), Name: "Espaço", Contracted: 6000, Payments: []domain.VendorPayment{{ID: uuid.New(), Amount: 6000, PaidOn: *vdate(2026, time.March, 1)}}},
			},
		},
	}
	s := newGoalsAPI(t, r) // hoje: 15/10/2026
	c := login(t, s)

	rec := do(s, "GET", "/api/vendors", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, body)
	}
	for _, want := range []string{
		`"contracted":18000`, `"paid":8000`,
		`"saved":2000`, `"savedFromBox":true`, // a caixinha "Casamento" somada entre as contas
		`"remaining":8000`, `"nextContribution":"2026-11-10"`, `"lastDue":"2027-08-20"`,
		`"monthsLeft":10`, `"perMonth":800`, `"boxes":["Casamento","Reserva"]`,
		`"description":"PIX BUFFET"`, `"paidOn":"2026-09-01"`, `"dueDate":null`, `"done":true`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("faltou %s em %s", want, body)
		}
	}
	if rec := do(s, "GET", "/api/vendors", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_CreateAndUpdateVendor(t *testing.T) {
	r := &fakeReader{}
	s := newGoalsAPI(t, r)
	c := login(t, s)
	json := map[string]string{"Content-Type": "application/json"}

	if rec := do(s, "POST", "/api/vendors", `{"name":" Churrasgrill ","contracted":18489.984,"dueDate":"2027-08-23","paid":4000}`, json, c); rec.Code != 201 {
		t.Fatalf("criar: %d %s", rec.Code, rec.Body)
	}
	v, p := r.vend.created[0], r.vend.initial[0]
	if v.Name != "Churrasgrill" || v.Contracted != 18489.98 || v.DueDate.Format(time.DateOnly) != "2027-08-23" {
		t.Errorf("fornecedor gravado: %+v", v)
	}
	if p == nil || p.Amount != 4000 || p.PaidOn.Format(time.DateOnly) != "2026-10-15" || p.VendorID != v.ID {
		t.Errorf("o já pago vira pagamento de hoje: %+v", p)
	}
	if rec := do(s, "POST", "/api/vendors", `{"name":"Espaço","contracted":6000}`, json, c); rec.Code != 201 || r.vend.created[1].DueDate != nil || r.vend.initial[1] != nil {
		t.Errorf("sem prazo e sem pago: %d %+v", rec.Code, r.vend.created[1])
	}

	for name, body := range map[string]string{
		"nome vazio":     `{"name":"  ","contracted":10}`,
		"valor zero":     `{"name":"a","contracted":0}`,
		"pago negativo":  `{"name":"a","contracted":10,"paid":-1}`,
		"data inválida":  `{"name":"a","contracted":10,"dueDate":"23/08/2027"}`,
		"data distante":  `{"name":"a","contracted":10,"dueDate":"2040-01-01"}`,
		"campo estranho": `{"name":"a","contracted":10,"x":1}`,
	} {
		if rec := do(s, "POST", "/api/vendors", body, json, c); rec.Code != 400 {
			t.Errorf("%s = %d", name, rec.Code)
		}
	}

	r.vend.found = true
	id := uuid.New()
	if rec := do(s, "PUT", "/api/vendors/"+id.String(), `{"name":"Buffet","contracted":13000,"dueDate":""}`, json, c); rec.Code != 200 || r.vend.updated[0].ID != id || r.vend.updated[0].DueDate != nil {
		t.Errorf("alterar: %d %+v", rec.Code, r.vend.updated)
	}
	r.vend.found = false
	if rec := do(s, "PUT", "/api/vendors/"+id.String(), `{"name":"Buffet","contracted":13000}`, json, c); rec.Code != 404 {
		t.Errorf("alterar inexistente = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/vendors/"+id.String(), "", nil, c); rec.Code != 404 {
		t.Errorf("apagar inexistente = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/vendors/"+id.String(), "", map[string]string{"Origin": "https://evil.example"}, c); rec.Code != 403 {
		t.Errorf("origem diferente = %d", rec.Code)
	}
}

func TestAPI_VendorPayments(t *testing.T) {
	vendorID := uuid.New()
	r := &fakeReader{vend: vendorFake{vendors: []domain.Vendor{{ID: vendorID}}}}
	s := newGoalsAPI(t, r)
	c := login(t, s)
	json := map[string]string{"Content-Type": "application/json"}
	path := "/api/vendors/" + vendorID.String() + "/payments"

	if rec := do(s, "POST", path, `{"amount":500.5,"paidOn":"2026-10-01"}`, json, c); rec.Code != 201 {
		t.Fatalf("à mão: %d %s", rec.Code, rec.Body)
	}
	if p := r.vend.payments[0]; p.Amount != 500.5 || p.PaidOn.Format(time.DateOnly) != "2026-10-01" || p.TransactionID != nil || p.VendorID != vendorID {
		t.Errorf("pagamento à mão: %+v", p)
	}
	txID := uuid.New()
	if rec := do(s, "POST", path, `{"transactionId":"`+txID.String()+`"}`, json, c); rec.Code != 201 || *r.vend.payments[1].TransactionID != txID {
		t.Errorf("ligado ao extrato: %d %+v", rec.Code, r.vend.payments[1])
	}

	for name, body := range map[string]string{
		"sem valor":    `{"paidOn":"2026-10-01"}`,
		"sem data":     `{"amount":10}`,
		"ambos":        `{"transactionId":"` + txID.String() + `","amount":10}`,
		"id de lixo":   `{"transactionId":"abc"}`,
		"valor enorme": `{"amount":2e9,"paidOn":"2026-10-01"}`,
	} {
		if rec := do(s, "POST", path, body, json, c); rec.Code != 400 {
			t.Errorf("%s = %d", name, rec.Code)
		}
	}

	for err, code := range map[error]int{domain.ErrVendorTxLinked: 409, domain.ErrVendorTxNotFound: 404, domain.ErrVendorNotFound: 404} {
		r.vend.addErr = err
		if rec := do(s, "POST", path, `{"transactionId":"`+txID.String()+`"}`, json, c); rec.Code != code || !strings.Contains(rec.Body.String(), err.Error()) {
			t.Errorf("%v = %d %s", err, rec.Code, rec.Body)
		}
	}

	r.vend.found = true
	if rec := do(s, "DELETE", path+"/"+uuid.New().String(), "", nil, c); rec.Code != 200 {
		t.Errorf("apagar pagamento = %d", rec.Code)
	}
}

func TestAPI_SaveVendorPlan(t *testing.T) {
	r := &fakeReader{}
	s := newGoalsAPI(t, r)
	c := login(t, s)
	json := map[string]string{"Content-Type": "application/json"}

	if rec := do(s, "PUT", "/api/vendors/plan", `{"name":"Casamento","contributionDay":10,"savingsBox":" Casamento ","savedManual":0}`, json, c); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if p := r.vend.plan; p.Name != "Casamento" || p.ContributionDay != 10 || p.SavingsBox != "Casamento" {
		t.Errorf("plano gravado: %+v", p)
	}
	for name, body := range map[string]string{
		"dia 31":       `{"name":"C","contributionDay":31}`,
		"dia 0":        `{"name":"C","contributionDay":0}`,
		"guardado < 0": `{"name":"C","contributionDay":5,"savedManual":-1}`,
		"nome vazio":   `{"name":"","contributionDay":5}`,
	} {
		if rec := do(s, "PUT", "/api/vendors/plan", body, json, c); rec.Code != 400 {
			t.Errorf("%s = %d", name, rec.Code)
		}
	}
}
