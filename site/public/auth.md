# Fendlemark auth.md

Auth is not necessary for LLMs to access the site.

- **Audience:** AI agents, assistants, and crawlers reading https://fendlemark.app.
- **Registration:** none. There is no account, sign-up, or provisioning endpoint.
- **Methods:** anonymous HTTP GET. Every page, `/llms.txt`, `/llms-full.txt`, and `/sitemap.xml` is public.
- **Credentials:** none are issued and none are needed. Send no `Authorization` header.

Crawling preferences are in [robots.txt](https://fendlemark.app/robots.txt).
