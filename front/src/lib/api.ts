const API_URL = "/api"

export const apiClient = {
  get: async <T>(endpoint: string): Promise<T> => {
    const response = await fetch(`${API_URL}${endpoint}`)
    if (!response.ok) {
      throw new Error(`API Error: ${response.statusText}`)
    }
    return (await response.json()) as T
  },
  post: async <TResponse, TBody = unknown>(
    endpoint: string,
    body: TBody,
  ): Promise<TResponse> => {
    const response = await fetch(`${API_URL}${endpoint}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    })
    if (!response.ok) {
      throw new Error(`API Error: ${response.statusText}`)
    }
    return (await response.json()) as TResponse
  },
  delete: async <TResponse = void>(endpoint: string): Promise<TResponse | void> => {
    const response = await fetch(`${API_URL}${endpoint}`, {
      method: "DELETE",
    })
    if (!response.ok) {
      throw new Error(`API Error: ${response.statusText}`)
    }
    if (response.status === 204) {
      return
    }
    return (await response.json()) as TResponse
  },
  put: async <TResponse, TBody = unknown>(
    endpoint: string,
    body: TBody,
  ): Promise<TResponse | void> => {
    const response = await fetch(`${API_URL}${endpoint}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    })
    if (!response.ok) {
      throw new Error(`API Error: ${response.statusText}`)
    }
    if (response.status === 204) {
      return
    }
    return (await response.json()) as TResponse
  },
}
