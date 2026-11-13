# BKL vacation portal

Russian and English portal for vacation, day off, and sick-day requests. Employees, managers, and HR use one app. An HR person still has the employee area for their own days.

Sign-in is the company’s existing Keycloak (authorization code with PKCE). The screens call the Go API. Until an OpenAPI document is in the repo, the calls live in `src/api/paths.ts` and `src/api/http.ts`.

## Scripts

```bash
npm install
npm test
npm run build
npm run dev
```

## Keycloak

Copy `.env.example` to `.env.local` and set the existing realm:

- `VITE_KEYCLOAK_URL`
- `VITE_KEYCLOAK_REALM`
- `VITE_KEYCLOAK_CLIENT_ID`
- `VITE_API_BASE` (default `/api`)

The client in Keycloak is a public client with the standard authorization-code flow and PKCE S256. There is no second login.

## Local preview

`VITE_PREVIEW=1 npm run dev` is a development-only view with sample data, so the screens can be clicked without the API. It is not a login and it is not included when `import.meta.env.DEV` is false.

## Brand

Leon red `#d71015`, yellow `#fdc000`, black `#0C1013`, and greys `#33363C` and `#6B737C` come from https://leon.graphics/. The shell is a liquid-glass window (macOS traffic lights, large radii, blur) on a cold black-and-grey stage. The stage photo is half transparent: a black-and-white ground with one man left in color, plus an ASCII texture. The signed-in name is white and the role is yellow. Interface type follows the system UI font, with Roboto as the bundled fallback (Apache license in `src/assets/fonts/Roboto-LICENSE.txt`).
