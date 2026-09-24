import { afterEach, describe, expect, it, vi } from "vitest";
import {
  clearTokens,
  clearUser,
  getUser,
  isAdmin,
  isAuthenticated,
  logout,
  saveUser,
} from "./auth";
import type { MeResponse, User } from "./types";

const KEY = "auth_user";

const baseUser: User = {
  id: 1,
  nome: "Ana",
  email: "ana@x.com",
  role: "normal",
  ativo: true,
  created_at: "2026-01-01",
  updated_at: "2026-01-02",
  id_vendedor: 7,
  vendedor_nome: "Vend 7",
};

function stored(): Record<string, unknown> {
  return JSON.parse(localStorage.getItem(KEY) ?? "null");
}

describe("saveUser", () => {
  it.each<[string, Partial<MeResponse>]>([
    ["vendedor_desligado: true", { vendedor_desligado: true }],
    ["vendedor_desligado: false", { vendedor_desligado: false }],
  ])("nao grava %s no localStorage", (_n, extra) => {
    saveUser({ ...baseUser, ...extra } as MeResponse);
    const s = stored();
    expect(s).not.toHaveProperty("vendedor_desligado");
    expect(localStorage.getItem(KEY)).not.toContain("vendedor_desligado");
  });

  it("grava apenas os campos conhecidos de User (whitelist)", () => {
    saveUser({
      ...baseUser,
      vendedor_desligado: true,
      senha_hash: "x",
      token: "y",
    } as unknown as User);
    expect(Object.keys(stored()).sort()).toEqual(
      [
        "ativo",
        "created_at",
        "email",
        "id",
        "id_vendedor",
        "nome",
        "role",
        "updated_at",
        "vendedor_nome",
      ].sort()
    );
    expect(stored()).toMatchObject({ id: 1, id_vendedor: 7, vendedor_nome: "Vend 7" });
  });

  it("normaliza id_vendedor/vendedor_nome ausentes para null", () => {
    const { id_vendedor: _a, vendedor_nome: _b, ...semVinculo } = baseUser;
    void _a;
    void _b;
    saveUser(semVinculo);
    expect(stored().id_vendedor).toBeNull();
    expect(stored().vendedor_nome).toBeNull();
  });
});

describe("getUser/clearUser/isAdmin/isAuthenticated", () => {
  it("retorna null sem usuario salvo", () => {
    expect(getUser()).toBeNull();
    expect(isAuthenticated()).toBe(false);
    expect(isAdmin()).toBe(false);
  });

  it("retorna null quando o JSON esta corrompido", () => {
    localStorage.setItem(KEY, "{nao-json");
    expect(getUser()).toBeNull();
  });

  it.each<[User["role"], boolean]>([
    ["admin", true],
    ["normal", false],
  ])("role %s -> isAdmin %s", (role, esperado) => {
    saveUser({ ...baseUser, role });
    expect(isAuthenticated()).toBe(true);
    expect(isAdmin()).toBe(esperado);
  });

  it.each([
    ["clearUser", clearUser],
    ["clearTokens", clearTokens],
  ])("%s remove o usuario do localStorage", (_n, fn) => {
    saveUser(baseUser);
    fn();
    expect(localStorage.getItem(KEY)).toBeNull();
  });
});

describe("logout", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each([
    ["backend OK", () => Promise.resolve(new Response(null, { status: 200 }))],
    ["falha de rede", () => Promise.reject(new TypeError("network"))],
  ])("limpa o usuario e chama POST /api/auth/logout (%s)", async (_n, impl) => {
    const fetchMock = vi.fn(impl);
    vi.stubGlobal("fetch", fetchMock);
    saveUser(baseUser);

    await expect(logout()).resolves.toBeUndefined();

    expect(localStorage.getItem(KEY)).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toMatch(/\/api\/auth\/logout$/);
    expect(init).toMatchObject({ method: "POST", credentials: "include" });
  });
});
