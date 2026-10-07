/**
 * Erro HTTP da API com o status preservado, para que as telas possam
 * distinguir casos especificos (ex.: 403 de vendedor desligado) sem
 * depender de comparacao por substring.
 */
import { vlog } from "./vlog";

export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    vlog("apiError.ts", "ApiError.constructor", "definindo name do erro da API");
    this.name = "ApiError";
    vlog("apiError.ts", "ApiError.constructor", "definindo status do erro da API:", status);
    this.status = status;
  }
}
