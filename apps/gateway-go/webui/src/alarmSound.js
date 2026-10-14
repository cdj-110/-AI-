const levelPriority = { INFO: 1, WARNING: 2, CRITICAL: 3 };
export function highestAlarmLevel(levels) {
    return levels.reduce((highest, level) => {
        const normalized = String(level || 'INFO').toUpperCase();
        return !highest || (levelPriority[normalized] || 0) > (levelPriority[highest] || 0) ? normalized : highest;
    }, undefined);
}
export function alarmTonePattern(level) {
    switch (String(level || '').toUpperCase()) {
        case 'CRITICAL':
            return [
                { frequency: 784, delaySeconds: 0, durationSeconds: 0.13 },
                { frequency: 988, delaySeconds: 0.18, durationSeconds: 0.13 },
                { frequency: 1175, delaySeconds: 0.36, durationSeconds: 0.17 },
            ];
        case 'WARNING':
            return [
                { frequency: 784, delaySeconds: 0, durationSeconds: 0.13 },
                { frequency: 988, delaySeconds: 0.19, durationSeconds: 0.15 },
            ];
        default:
            return [{ frequency: 880, delaySeconds: 0, durationSeconds: 0.14 }];
    }
}
export class AlarmSoundPlayer {
    context;
    pendingLevel;
    play(level) {
        const context = this.ensureContext();
        if (!context || context.state !== 'running') {
            this.pendingLevel = highestAlarmLevel([this.pendingLevel || '', level]) || level;
            return false;
        }
        this.pendingLevel = undefined;
        this.emit(context, level);
        return true;
    }
    async unlock() {
        const context = this.ensureContext();
        if (!context)
            return false;
        try {
            if (context.state === 'suspended')
                await context.resume();
        }
        catch {
            return false;
        }
        if (context.state !== 'running')
            return false;
        if (this.pendingLevel) {
            const level = this.pendingLevel;
            this.pendingLevel = undefined;
            this.emit(context, level);
        }
        return true;
    }
    close() {
        const context = this.context;
        this.context = undefined;
        this.pendingLevel = undefined;
        if (context && context.state !== 'closed')
            void context.close().catch(() => undefined);
    }
    ensureContext() {
        if (this.context)
            return this.context;
        if (typeof window === 'undefined')
            return undefined;
        const AudioContextConstructor = window.AudioContext
            || window.webkitAudioContext;
        if (!AudioContextConstructor)
            return undefined;
        try {
            this.context = new AudioContextConstructor();
        }
        catch {
            return undefined;
        }
        return this.context;
    }
    emit(context, level) {
        const startAt = context.currentTime + 0.02;
        const volume = String(level).toUpperCase() === 'CRITICAL' ? 0.11 : 0.075;
        for (const tone of alarmTonePattern(level)) {
            const oscillator = context.createOscillator();
            const gain = context.createGain();
            const toneStart = startAt + tone.delaySeconds;
            const toneEnd = toneStart + tone.durationSeconds;
            oscillator.type = 'sine';
            oscillator.frequency.setValueAtTime(tone.frequency, toneStart);
            gain.gain.setValueAtTime(0.0001, toneStart);
            gain.gain.exponentialRampToValueAtTime(volume, toneStart + 0.018);
            gain.gain.exponentialRampToValueAtTime(0.0001, toneEnd);
            oscillator.connect(gain);
            gain.connect(context.destination);
            oscillator.start(toneStart);
            oscillator.stop(toneEnd + 0.02);
        }
    }
}
