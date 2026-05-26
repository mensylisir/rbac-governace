import Keycloak from 'keycloak-js'

const keycloak = new Keycloak({
  url: 'https://keycloak-dev.rdev.tech/auth',
  realm: 'project',
  clientId: 'rbac-manager',
})

export async function initKeycloak(): Promise<boolean> {
  try {
    const isSecure = typeof window !== 'undefined' && (window.isSecureContext || window.location.hostname === 'localhost')
    const authenticated = await keycloak.init({
      onLoad: 'login-required',
      checkLoginIframe: false,
      pkceMethod: isSecure ? 'S256' : false,
    })
    return authenticated
  } catch (err) {
    console.error('Keycloak init failed:', err)
    return false
  }
}

export function getToken(): string | undefined {
  return keycloak.token
}

export async function updateToken(minValidity = 30): Promise<string | undefined> {
  try {
    const refreshed = await keycloak.updateToken(minValidity)
    if (refreshed) {
      console.log('Token refreshed')
    }
    return keycloak.token
  } catch (err) {
    console.error('Token refresh failed:', err)
    keycloak.login()
    return undefined
  }
}

export function logout(): void {
  keycloak.logout({ redirectUri: window.location.origin })
}

export function getUsername(): string | undefined {
  return keycloak.tokenParsed?.preferred_username as string
    || keycloak.tokenParsed?.name as string
    || keycloak.tokenParsed?.sub as string
}

export function getUserId(): string | undefined {
  return keycloak.tokenParsed?.sub as string
}

export function getGroups(): string[] {
  const groups = keycloak.tokenParsed?.groups as string[] || []
  const realmRoles = (keycloak.tokenParsed?.realm_access as any)?.roles as string[] || []
  return [...groups.map((g: string) => g.replace(/^\//, '')), ...realmRoles]
}

export default keycloak
