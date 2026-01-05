const API_URL = '/api';

export const apiClient = {
  get: async (endpoint: string) => {
    const response = await fetch(`${API_URL}${endpoint}`);
    if (!response.ok) {
        throw new Error(`API Error: ${response.statusText}`);
    }
    return response.json();
  },
  post: async (endpoint: string, body: any) => {
    const response = await fetch(`${API_URL}${endpoint}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
    });
    if (!response.ok) {
        throw new Error(`API Error: ${response.statusText}`)
    }
    return response.json();
  },
  delete: async (endpoint: string) => {
    const response = await fetch(`${API_URL}${endpoint}`, {
        method: 'DELETE',
    });
    if (!response.ok) {
        throw new Error(`API Error: ${response.statusText}`);
    }
    if (response.status === 204) {
      return;
    }
    return response.json();
  },
  put: async (endpoint: string, body: any) => {
    const response = await fetch(`${API_URL}${endpoint}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
    });
    if (!response.ok) {
        throw new Error(`API Error: ${response.statusText}`)
    }
    if (response.status === 204) {
      return;
    }
    return response.json();
  }
}