package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func SetSEORouter(router *gin.Engine, siteRuntime SiteRuntimeConfig) {
	router.GET("/robots.txt", func(c *gin.Context) {
		if siteRuntime.CanonicalOrigin == "" {
			c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("User-agent: *\nDisallow: /\n"))
			return
		}
		c.Data(
			http.StatusOK,
			"text/plain; charset=utf-8",
			[]byte("User-agent: *\nDisallow: /api/\nDisallow: /v1/\nDisallow: /setup/\nDisallow: /_authenticated/\nSitemap: "+siteRuntime.CanonicalOrigin+"/sitemap.xml\n"),
		)
	})

	router.GET("/sitemap.xml", func(c *gin.Context) {
		if siteRuntime.CanonicalOrigin == "" {
			c.Data(http.StatusServiceUnavailable, "text/plain; charset=utf-8", []byte("sitemap is not configured\n"))
			return
		}

		urls := []string{
			siteRuntime.CanonicalOrigin + "/",
			siteRuntime.CanonicalOrigin + "/about/",
			siteRuntime.CanonicalOrigin + "/privacy-policy",
			siteRuntime.CanonicalOrigin + "/user-agreement",
		}
		entries := make([]string, 0, len(urls))
		for _, siteURL := range urls {
			entries = append(entries, "  <url><loc>"+siteURL+"</loc></url>")
		}
		body := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n" + strings.Join(entries, "\n") + "\n</urlset>\n"
		c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(body))
	})
}
