package names

// Popular is a small, original list of well-known names used to warm
// resolver caches. It is not copied from any third-party benchmark product.
var Popular = []string{
	"cloudflare.com",
	"google.com",
	"amazon.com",
	"microsoft.com",
	"apple.com",
	"wikipedia.org",
	"github.com",
	"iana.org",
	"kernel.org",
	"golang.org",
	"ubuntu.com",
	"debian.org",
	"mozilla.org",
	"ietf.org",
	"example.com",
	"cloudflare-dns.com",
	"quad9.net",
	"dns.google",
	"opendns.com",
	"wikimedia.org",
	"isc.org",
	"ripe.net",
	"verisign.com",
	"letsencrypt.org",
}

// Take returns the first n names, or the full list if n >= len(Popular).
// n < 1 returns the full list.
func Take(n int) []string {
	if n < 1 || n >= len(Popular) {
		out := make([]string, len(Popular))
		copy(out, Popular)
		return out
	}
	out := make([]string, n)
	copy(out, Popular[:n])
	return out
}
