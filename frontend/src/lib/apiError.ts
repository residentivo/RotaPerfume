/**
 * Erro HTTP da API com o status preservado, para que as telas possam
 * distinguir casos especificos (ex.: 403 de vendedor desligado) sem
 * depender de comparacao por substring.
 */
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}
