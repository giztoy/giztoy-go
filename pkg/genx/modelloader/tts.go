package modelloader

import (
	"fmt"
	"strings"

	doubaospeech "github.com/giztoy/doubao-speech-go"
	minimax "github.com/giztoy/minimax-go"
	"github.com/giztoy/giztoy-go/pkg/genx/transformers"
)

func registerTTSBySchema(cfg ConfigFile) ([]string, error) {
	// Parse schema to determine provider
	parts := strings.Split(cfg.Schema, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid schema: %s", cfg.Schema)
	}
	provider := parts[0]

	switch provider {
	case "doubao":
		return registerDoubaoTTS(cfg)
	case "minimax":
		return registerMinimaxTTS(cfg)
	default:
		return nil, fmt.Errorf("unknown TTS provider: %s", provider)
	}
}

func registerDoubaoTTS(cfg ConfigFile) ([]string, error) {
	if cfg.AppID == "" {
		return nil, fmt.Errorf("app_id is required for doubao TTS")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("token is required for doubao TTS")
	}

	// Create Doubao client
	client := doubaospeech.NewClient(cfg.AppID, doubaospeech.WithBearerToken(cfg.Token))

	// Parse default params
	var opts []transformers.DoubaoTTSSeedV2Option
	if cfg.DefaultParams != nil {
		if format, ok := cfg.DefaultParams["format"].(string); ok && format != "" {
			opts = append(opts, transformers.WithDoubaoTTSSeedV2Format(format))
		}
		if sampleRate, ok := cfg.DefaultParams["sample_rate"].(float64); ok && sampleRate > 0 {
			opts = append(opts, transformers.WithDoubaoTTSSeedV2SampleRate(int(sampleRate)))
		}
	}

	var names []string
	for _, v := range cfg.Voices {
		if v.Name == "" || v.VoiceID == "" {
			return nil, fmt.Errorf("voice entry missing name or voice_id")
		}

		// Use DoubaoTTSSeedV2 for all voices
		// The transformer will auto-detect resource ID based on voice suffix
		tts := transformers.NewDoubaoTTSSeedV2(client, v.VoiceID, opts...)
		// Register to both TTSMux and DefaultMux for compatibility
		transformers.HandleTTS(v.Name, tts)
		transformers.Handle(v.Name, tts)
		names = append(names, v.Name)
	}
	return names, nil
}

func registerMinimaxTTS(cfg ConfigFile) ([]string, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required for minimax TTS")
	}

	clientConfig := minimax.Config{APIKey: cfg.APIKey}
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	client, err := minimax.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("create minimax client: %w", err)
	}

	var names []string
	for _, v := range cfg.Voices {
		if v.Name == "" || v.VoiceID == "" {
			return nil, fmt.Errorf("voice entry missing name or voice_id")
		}

		ttsOpts := []transformers.MinimaxTTSOption{}
		if cfg.Model != "" {
			ttsOpts = append(ttsOpts, transformers.WithMinimaxTTSModel(cfg.Model))
		}

		tts := transformers.NewMinimaxTTS(client, v.VoiceID, ttsOpts...)
		// Register to both TTSMux and DefaultMux for compatibility
		transformers.HandleTTS(v.Name, tts)
		transformers.Handle(v.Name, tts)
		names = append(names, v.Name)
	}
	return names, nil
}
