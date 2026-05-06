'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { synthesizeVoicevoxSpeech } from '@/lib/backend-api';

type VoicevoxSpeechState = {
  supported: boolean;
  enabled: boolean;
  speaking: boolean;
  errorMessage: string;
  setEnabled: (enabled: boolean) => void;
  speak: (text: string) => void;
  enqueueStreamingText: (text: string) => void;
  finishStreamingText: () => void;
  stop: () => void;
};

function hasAudioPlayback(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof Audio !== 'undefined' &&
    typeof window.URL !== 'undefined'
  );
}

function extractSpeechSegments(buffer: string, force = false): { segments: string[]; rest: string } {
  const segments: string[] = [];
  let start = 0;

  for (let index = 0; index < buffer.length; index += 1) {
    if (!'。！？!?'.includes(buffer[index])) {
      continue;
    }

    const segment = buffer.slice(start, index + 1).trim();
    if (segment !== '') {
      segments.push(segment);
    }
    start = index + 1;
  }

  const rest = buffer.slice(start).trimStart();
  if (force && rest.trim() !== '') {
    return { segments: [...segments, rest.trim()], rest: '' };
  }

  return { segments, rest };
}

function joinSpeechSegments(segments: string[]): string {
  return segments.join('');
}

export function useVoicevoxSpeech(token: string): VoicevoxSpeechState {
  const [supported, setSupported] = useState(false);
  const [enabled, setEnabledState] = useState(false);
  const [speaking, setSpeaking] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const objectUrlRef = useRef<string | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const requestIDRef = useRef(0);
  const speechQueueRef = useRef<string[]>([]);
  const streamBufferRef = useRef('');
  const pendingSpeechSegmentsRef = useRef<string[]>([]);
  const processingQueueRef = useRef(false);

  const releaseObjectURL = useCallback(() => {
    if (objectUrlRef.current) {
      window.URL.revokeObjectURL(objectUrlRef.current);
      objectUrlRef.current = null;
    }
  }, []);

  const stop = useCallback(() => {
    requestIDRef.current += 1;
    speechQueueRef.current = [];
    streamBufferRef.current = '';
    pendingSpeechSegmentsRef.current = [];
    processingQueueRef.current = false;
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;

    if (audioRef.current) {
      audioRef.current.pause();
      audioRef.current.currentTime = 0;
      audioRef.current = null;
    }

    if (hasAudioPlayback()) {
      releaseObjectURL();
    }
    setSpeaking(false);
  }, [releaseObjectURL]);

  useEffect(() => {
    setSupported(hasAudioPlayback());
    return () => {
      stop();
    };
  }, [stop]);

  const setEnabled = useCallback((nextEnabled: boolean) => {
    setEnabledState(nextEnabled);
    setErrorMessage('');

    if (!nextEnabled) {
      stop();
    }
  }, [stop]);

  const playQueuedSpeech = useCallback(() => {
    if (processingQueueRef.current) {
      return;
    }
    if (!enabled || !supported || !token) {
      speechQueueRef.current = [];
      streamBufferRef.current = '';
      pendingSpeechSegmentsRef.current = [];
      setSpeaking(false);
      return;
    }

    const nextText = speechQueueRef.current.shift();
    if (!nextText) {
      setSpeaking(false);
      return;
    }

    processingQueueRef.current = true;
    const requestID = requestIDRef.current + 1;
    requestIDRef.current = requestID;
    const abortController = new AbortController();
    abortControllerRef.current = abortController;
    setSpeaking(true);
    setErrorMessage('');

    void (async () => {
      try {
        const blob = await synthesizeVoicevoxSpeech(token, nextText, abortController.signal);
        if (requestIDRef.current !== requestID || abortController.signal.aborted) {
          return;
        }

        releaseObjectURL();
        const objectURL = window.URL.createObjectURL(blob);
        objectUrlRef.current = objectURL;
        const audio = new Audio(objectURL);
        audioRef.current = audio;

        const completeCurrent = () => {
          if (requestIDRef.current !== requestID) {
            return;
          }
          audioRef.current = null;
          releaseObjectURL();
          abortControllerRef.current = null;
          processingQueueRef.current = false;
          playQueuedSpeech();
        };

        audio.onended = completeCurrent;
        audio.onerror = () => {
          if (requestIDRef.current === requestID) {
            setErrorMessage('VOICEVOX音声の再生に失敗しました');
          }
          completeCurrent();
        };

        await audio.play();
      } catch (error) {
        if (abortController.signal.aborted || requestIDRef.current !== requestID) {
          return;
        }
        audioRef.current = null;
        releaseObjectURL();
        abortControllerRef.current = null;
        processingQueueRef.current = false;
        setErrorMessage(error instanceof Error ? error.message : 'VOICEVOX音声の生成に失敗しました');
        playQueuedSpeech();
      }
    })();
  }, [enabled, releaseObjectURL, supported, token]);

  const enqueueSpeech = useCallback((text: string) => {
    const trimmed = text.trim();
    if (!enabled || !supported || trimmed === '' || !token) {
      return;
    }

    speechQueueRef.current.push(trimmed);
    playQueuedSpeech();
  }, [enabled, playQueuedSpeech, supported, token]);

  const speak = useCallback((text: string) => {
    stop();
    enqueueSpeech(text);
  }, [enqueueSpeech, stop]);

  const enqueueStreamingText = useCallback((text: string) => {
    if (!enabled || !supported || !token || text === '') {
      return;
    }

    streamBufferRef.current += text;
    const { segments, rest } = extractSpeechSegments(streamBufferRef.current);
    streamBufferRef.current = rest;
    pendingSpeechSegmentsRef.current.push(...segments);

    while (pendingSpeechSegmentsRef.current.length >= 2) {
      const batch = pendingSpeechSegmentsRef.current.splice(0, 2);
      enqueueSpeech(joinSpeechSegments(batch));
    }
  }, [enabled, enqueueSpeech, supported, token]);

  const finishStreamingText = useCallback(() => {
    if (!enabled || !supported || !token) {
      streamBufferRef.current = '';
      pendingSpeechSegmentsRef.current = [];
      return;
    }

    const { segments, rest } = extractSpeechSegments(streamBufferRef.current, true);
    streamBufferRef.current = rest;
    pendingSpeechSegmentsRef.current.push(...segments);
    if (pendingSpeechSegmentsRef.current.length > 0) {
      enqueueSpeech(joinSpeechSegments(pendingSpeechSegmentsRef.current));
    }
    pendingSpeechSegmentsRef.current = [];
  }, [enabled, enqueueSpeech, supported, token]);

  return {
    supported,
    enabled,
    speaking,
    errorMessage,
    setEnabled,
    speak,
    enqueueStreamingText,
    finishStreamingText,
    stop,
  };
}
