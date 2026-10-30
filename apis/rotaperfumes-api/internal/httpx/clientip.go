package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/rotaperfumes/shared/vlog"
)

const arqClientIP = "clientip.go"

// IPDoCliente devolve o IP de origem (chaves de rate limit e auditoria,
// I-14). X-Forwarded-For só vale com TRUST_PROXY_HEADERS=true (atrás de
// proxy confiável): por padrão é ignorado, porque o cliente o forja. Do XFF
// vale o item mais à direita da última linha (o que o proxy confiável viu,
// SEC-20); vazio ou inválido não recua para o anterior: cai no RemoteAddr.
// X-Real-IP nunca é lido. O IP sai normalizado (sem porta, sem zona, IPv4
// mapeado em IPv6 vira IPv4).
func IPDoCliente(r *http.Request, confiarProxy bool) string {
	if confiarProxy {
		if ip, ok := ipDoXFF(r); ok {
			return ip
		}
	}
	return ipDoRemoteAddr(r.RemoteAddr)
}

// ipDoXFF pega o último item da última linha de X-Forwarded-For e o valida
// como IP sem porta. ok=false se não houver header ou o item for inválido.
func ipDoXFF(r *http.Request) (string, bool) {
	linhas := r.Header.Values("X-Forwarded-For")
	if len(linhas) == 0 {
		return "", false
	}
	itens := strings.Split(linhas[len(linhas)-1], ",")
	ultimo := strings.TrimSpace(itens[len(itens)-1])
	if ip, ok := normalizarIP(ultimo); ok {
		return ip, true
	}
	vlog.Printf(arqClientIP, "IPDoCliente", "X-Forwarded-For com último item vazio ou inválido: usando RemoteAddr")
	return "", false
}

// ipDoRemoteAddr tira a porta do RemoteAddr e normaliza; se não for IP,
// devolve o RemoteAddr como veio.
func ipDoRemoteAddr(remoto string) string {
	host, _, err := net.SplitHostPort(remoto)
	if err != nil {
		host = remoto
	}
	if ip, ok := normalizarIP(host); ok {
		return ip
	}
	return host
}

// normalizarIP valida s como endereço IP (sem porta) e devolve a forma
// canônica: IPv4 mapeado vira IPv4 e a zona é descartada.
func normalizarIP(s string) (string, bool) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return "", false
	}
	return addr.Unmap().WithZone("").String(), true
}
