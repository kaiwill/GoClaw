export const TOKEN_STORAGE_KEY = 'zeroclaw_token';
export const USER_STORAGE_KEY = 'zeroclaw_user';

let inMemoryToken: string | null = null;
let inMemoryUser: any = null;

function readStorage(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeStorage(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // localStorage may be unavailable in some browser privacy modes
  }
}

function removeStorage(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    // Ignore
  }
}

function clearLegacyLocalStorageToken(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    // Ignore
  }
}

export function getToken(): string | null {
  if (inMemoryToken && inMemoryToken.length > 0) {
    return inMemoryToken;
  }

  const storedToken = readStorage(TOKEN_STORAGE_KEY);
  if (storedToken && storedToken.length > 0) {
    inMemoryToken = storedToken;
    return storedToken;
  }

  return null;
}

export function setToken(token: string): void {
  inMemoryToken = token;
  writeStorage(TOKEN_STORAGE_KEY, token);
}

export function clearToken(): void {
  inMemoryToken = null;
  removeStorage(TOKEN_STORAGE_KEY);
  clearLegacyLocalStorageToken(TOKEN_STORAGE_KEY);
}

export function isAuthenticated(): boolean {
  const token = getToken();
  return token !== null && token.length > 0;
}

export function getUser(): any {
  if (inMemoryUser) {
    return inMemoryUser;
  }

  try {
    const userStr = localStorage.getItem(USER_STORAGE_KEY);
    if (userStr) {
      inMemoryUser = JSON.parse(userStr);
      return inMemoryUser;
    }
  } catch {
    // Ignore
  }

  return null;
}

export function setUser(user: any): void {
  inMemoryUser = user;
  try {
    localStorage.setItem(USER_STORAGE_KEY, JSON.stringify(user));
  } catch {
    // localStorage may be unavailable in some browser privacy modes
  }
}

export function clearUser(): void {
  inMemoryUser = null;
  try {
    localStorage.removeItem(USER_STORAGE_KEY);
  } catch {
    // Ignore
  }
}
