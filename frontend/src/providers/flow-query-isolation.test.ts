import type { DocumentNode, FieldNode, OperationDefinitionNode } from 'graphql';

import { describe, expect, it } from 'vitest';

import {
    AgentLogsDocument,
    FlowDocument,
    MessageLogsDocument,
    ScreenshotsDocument,
    SearchLogsDocument,
    TasksDocument,
    TerminalLogsDocument,
    VectorStoreLogsDocument,
} from '@/graphql/types';

function topLevelFields(document: DocumentNode): Array<string> {
    const operation = document.definitions.find(
        (definition): definition is OperationDefinitionNode => definition.kind === 'OperationDefinition',
    );

    return (
        operation?.selectionSet.selections
            .filter((selection): selection is FieldNode => selection.kind === 'Field')
            .map((selection) => selection.name.value) ?? []
    );
}

describe('flow detail queries', () => {
    it('keeps the flow summary independent from potentially large history responses', () => {
        expect(topLevelFields(FlowDocument)).toEqual(['flow']);
    });

    it.each([
        ['tasks', TasksDocument],
        ['screenshots', ScreenshotsDocument],
        ['terminalLogs', TerminalLogsDocument],
        ['messageLogs', MessageLogsDocument],
        ['agentLogs', AgentLogsDocument],
        ['searchLogs', SearchLogsDocument],
        ['vectorStoreLogs', VectorStoreLogsDocument],
    ])('loads %s in an isolated request', (field, document) => {
        expect(topLevelFields(document)).toEqual([field]);
    });
});
