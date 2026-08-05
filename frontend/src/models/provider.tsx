import { ProviderType } from '@/graphql/types';

export interface Provider {
    model?: null | string;
    name: string;
    type: ProviderType;
}

/**
 * Generates a display name for a provider.
 *
 * A "custom" provider is a generic OpenAI-compatible gateway, so its name
 * ("custom") is meaningless to the user — show the configured model instead
 * (e.g. the LLM_SERVER_MODEL chosen at install). Built-in providers keep their
 * own name.
 */
export const getProviderDisplayName = (provider: Provider): string => {
    if (provider.type === ProviderType.Custom && provider.model) {
        return provider.model;
    }

    return provider.name;
};

/**
 * Checks if a provider exists in the list of providers
 */
export const isProviderValid = (provider: Provider, providers: Provider[]): boolean => {
    return providers.some((p) => p.name === provider.name && p.type === provider.type);
};

/**
 * Finds a provider by name and type
 */
export const findProvider = (provider: Provider, providers: Provider[]): Provider | undefined => {
    return providers.find((p) => p.name === provider.name && p.type === provider.type);
};

/**
 * Finds a provider by name
 */
export const findProviderByName = (providerName: string, providers: Provider[]): Provider | undefined => {
    return providers.find((provider) => provider.name === providerName);
};

/**
 * Sorts providers by name alphabetically
 */
export const sortProviders = (providers: Provider[]): Provider[] => {
    return [...providers].sort((a, b) => a.name.localeCompare(b.name));
};
