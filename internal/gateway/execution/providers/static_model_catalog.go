package providers

type StaticModelCatalogEntry struct {
	ChannelName string
	ModelList   []string
}

func StaticModelCatalogEntries() []StaticModelCatalogEntry {
	return []StaticModelCatalogEntry{
		{
			ChannelName: "ai360",
			ModelList: []string{
				"360gpt-turbo",
				"360gpt-turbo-responsibility-8k",
				"360gpt-pro",
				"360gpt2-pro",
				"360GPT_S2_V9",
				"embedding-bert-512-v1",
				"embedding_s1_v1",
				"semantic_similarity_s1_v1",
			},
		},
		{
			ChannelName: "moonshot",
			ModelList: []string{
				"kimi-k2.5",
				"kimi-k2-0905-preview",
				"kimi-k2-turbo-preview",
				"kimi-k2-thinking",
				"kimi-k2-thinking-turbo",
			},
		},
		{
			ChannelName: "lingyiwanwu",
			ModelList: []string{
				"yi-large",
				"yi-medium",
				"yi-vision",
				"yi-medium-200k",
				"yi-spark",
				"yi-large-rag",
				"yi-large-turbo",
				"yi-large-preview",
				"yi-large-rag-preview",
			},
		},
		{
			ChannelName: "minimax",
			ModelList: []string{
				"abab6.5-chat",
				"abab6.5s-chat",
				"abab6-chat",
				"abab5.5-chat",
				"abab5.5s-chat",
				"MiniMax-M2.7",
				"MiniMax-M2.7-highspeed",
				"speech-2.5-hd-preview",
				"speech-2.5-turbo-preview",
				"speech-02-hd",
				"speech-02-turbo",
				"speech-01-hd",
				"speech-01-turbo",
				"MiniMax-M2.1",
				"MiniMax-M2.1-highspeed",
				"MiniMax-M2",
				"MiniMax-M2.5",
				"MiniMax-M2.5-highspeed",
				"image-01",
				"image-01-live",
			},
		},
	}
}
