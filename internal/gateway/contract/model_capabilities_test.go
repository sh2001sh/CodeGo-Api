package contract

import "testing"

func TestIsOpenAIResponseOnlyModel(t *testing.T) {
	t.Parallel()

	if !IsOpenAIResponseOnlyModel("o3-pro") {
		t.Fatal("expected o3-pro to require responses API")
	}
	if IsOpenAIResponseOnlyModel("gpt-4o-mini") {
		t.Fatal("did not expect gpt-4o-mini to require responses API")
	}
}

func TestIsImageGenerationModel(t *testing.T) {
	t.Parallel()

	if !IsImageGenerationModel("gpt-image-1") {
		t.Fatal("expected gpt-image-1 to be treated as image generation")
	}
	if !IsImageGenerationModel("gpt-image-2") {
		t.Fatal("expected gpt-image-2 to be treated as image generation")
	}
	if !IsImageGenerationModel("imagen-3.0-generate") {
		t.Fatal("expected imagen prefix to be treated as image generation")
	}
	if IsImageGenerationModel("gpt-4o-mini") {
		t.Fatal("did not expect gpt-4o-mini to be treated as image generation")
	}
}

func TestIsOpenAITextModel(t *testing.T) {
	t.Parallel()

	if !IsOpenAITextModel("gpt-4o-mini") {
		t.Fatal("expected gpt-4o-mini to use OpenAI text tokenizer")
	}
	if !IsOpenAITextModel("ChatGPT-4o") {
		t.Fatal("expected chatgpt family to use OpenAI text tokenizer")
	}
	if IsOpenAITextModel("claude-3-5-sonnet") {
		t.Fatal("did not expect claude to use OpenAI text tokenizer")
	}
}
