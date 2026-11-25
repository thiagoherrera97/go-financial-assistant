package httpserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/auth"
)

// login confere a senha. Com o segundo fator ativo, não abre a sessão: manda o código ao chat e prende o desafio a este
// navegador (cookie fa_challenge); a sessão só sai em loginCode.
func (a *api) login(w http.ResponseWriter, r *http.Request) {
	// JSON obrigatório + mesma origem: um formulário de outro site não consegue fazer login em nome do usuário (CSRF).
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, maxLoginBody, &body) {
		return
	}
	if !a.sessions.checkPassword(body.Password) {
		a.logger.Warn("login do dashboard recusado", "ip", clientIP(r))
		writeError(w, http.StatusUnauthorized, "senha incorreta")
		return
	}
	if !a.factor.Active() {
		a.loggedIn(w, r)
		return
	}
	bind, err := randomToken()
	if err != nil {
		a.fail(w, "gerar desafio", err)
		return
	}
	if !a.sendCode(w, r, auth.Login, bind, loginCodeMessage, nil) {
		return
	}
	setChallengeCookie(w, r, bind, int(auth.CodeTTL.Seconds()))
	writeJSON(w, http.StatusOK, map[string]string{"step": "code"})
}

// loginCode confere o código enviado ao chat e, se bater, abre a sessão.
func (a *api) loginCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, maxLoginBody, &body) {
		return
	}
	c, err := r.Cookie(challengeCookie)
	if err != nil {
		writeError(w, http.StatusUnauthorized, auth.ErrNoChallenge.Error())
		return
	}
	if err := a.challenges.Verify(auth.Login, c.Value, strings.TrimSpace(body.Code)); err != nil {
		a.codeRejected(w, r, err, http.StatusUnauthorized, "no login")
		return
	}
	setChallengeCookie(w, r, "", -1)
	a.loggedIn(w, r)
}

func (a *api) loggedIn(w http.ResponseWriter, r *http.Request) {
	a.sessions.issue(w, r)
	a.notify(r.Context(), "🔓 Login no dashboard (IP "+clientIP(r)+", "+time.Now().Format("02/01 15:04")+").")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	a.sessions.clear(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// me diz se há sessão e se o segundo fator está ativo (a tela de Configurações avisa quando não está).
func (a *api) me(w http.ResponseWriter, _ *http.Request) {
	// cycle é o ciclo do salário de hoje: o "mês atual" das telas (pode ser o mês anterior no calendário).
	month := a.monthStart()
	start, end := a.insights.Cycles().Bounds(month)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "secondFactor": a.factor.Active(), "noAuth": a.sessions.open, "name": a.ownerName(), "cycle": map[string]string{
		"month": formatMonth(month), "startsOn": start.Format("2006-01-02"), "endsOn": end.Format("2006-01-02"),
	}})
}

// ownerName é o nome de quem usa esta instalação (DISPLAY_NAME), ou o primeiro nome do primeiro de OWN_NAMES; "" sem nenhum.
func (a *api) ownerName() string {
	if a.settings == nil || a.settings.Service == nil {
		return ""
	}
	if n := strings.TrimSpace(a.settings.Service.Get("DISPLAY_NAME")); n != "" {
		return n
	}
	for _, full := range a.settings.Service.List("OWN_NAMES") {
		if f := strings.Fields(full); len(f) > 0 {
			r := []rune(strings.ToLower(f[0]))
			return strings.ToUpper(string(r[:1])) + string(r[1:])
		}
	}
	return ""
}

func (a *api) registerAuth(rt routes, loginLimiter *ipRateLimiter) {
	rt.mux.Handle("POST /api/login", loginLimiter.middleware(http.HandlerFunc(jsonOnly(a.login))))
	// Cada desafio aceita poucas tentativas; o limite por IP é só uma segunda barreira.
	rt.mux.Handle("POST /api/login/code", newIPRateLimiter(10, time.Minute).middleware(http.HandlerFunc(jsonOnly(a.loginCode))))
	rt.mux.Handle("POST /api/logout", rt.protected(sameOriginOnly(a.logout)))
	rt.mux.Handle("GET /api/me", rt.protected(a.me))
}
