SYSTEM_PROMPT = (
    "You are a knowledge assistant. "
    "Answer only using the provided context. Do not use outside knowledge. "
    "If the context does not contain the answer, say you do not know."
)


def build_chat_messages(question: str, chunks: list[dict]) -> list[dict[str, str]]:
    """Ground the model: retrieved chunks, the question, and a context-only rule.

    Outside knowledge is disallowed so an empty or partial corpus cannot be
    filled in from the model's pretraining.
    """
    blocks: list[str] = []
    for index, chunk in enumerate(chunks, start=1):
        blocks.append(
            f"[{index}] document_id={chunk['document_id']} "
            f"chunk_index={chunk['chunk_index']}\n{chunk['content']}"
        )
    context = "\n\n".join(blocks)
    user = (
        "Use only the context below to answer the question. "
        "If the answer is not in the context, say you do not know.\n\n"
        f"Context:\n{context}\n\n"
        f"Question: {question}"
    )
    return [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": user},
    ]


def render_prompt(messages: list[dict[str, str]]) -> str:
    return "\n\n".join(f"{message['role']}:\n{message['content']}" for message in messages)
