let refreshInterval = null;
let lastLogContent = "";

function escapeHTML(str) {
    return str.replace(/[&<>'"]/g,
        tag => ({
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            "'": '&#39;',
            '"': '&quot;'
        }[tag] || tag)
    );
}

// Analisa e colore as linhas de logs para simular um terminal com sintaxe destacada
function formatLogLine(line) {
    const trimmed = line.trim();
    if (!trimmed) return "";

    let colorClass = "text-zinc-400";
    let escaped = escapeHTML(line);

    // Verifica palavras-chave e define estilos de cor
    if (escaped.includes("🚨") || escaped.toLowerCase().includes("erro") || escaped.toLowerCase().includes("error") || escaped.toLowerCase().includes("failed") || escaped.toLowerCase().includes("panic") || escaped.toLowerCase().includes("fatal")) {
        colorClass = "text-rose-400 font-semibold";
    } else if (escaped.includes("✅") || escaped.toLowerCase().includes("sucesso") || escaped.toLowerCase().includes("success") || escaped.toLowerCase().includes("conectado") || escaped.toLowerCase().includes("connected")) {
        colorClass = "text-emerald-400 font-semibold";
    } else if (escaped.includes("⚠️") || escaped.toLowerCase().includes("warning") || escaped.toLowerCase().includes("warn") || escaped.toLowerCase().includes("offline") || escaped.toLowerCase().includes("desconectado")) {
        colorClass = "text-amber-400 font-semibold";
    } else if (escaped.includes("[POLL]") || escaped.includes("[VOTO]")) {
        colorClass = "text-zinc-300";
    } else if (escaped.includes("[WEBHOOK]") || escaped.includes("[API]")) {
        colorClass = "text-zinc-200";
    } else if (escaped.includes("[TICKET]") || escaped.includes("[GLPI]")) {
        colorClass = "text-white font-medium";
    } else if (escaped.includes("🔄") || escaped.includes("iniciado") || escaped.includes("reiniciado") || escaped.includes("START")) {
        colorClass = "text-zinc-100 font-semibold";
    }

    return `<div class="${colorClass} py-0.5">${escaped}</div>`;
}

async function fetchLogs() {
    const limitSelect = document.getElementById('limitSelect');
    if (!limitSelect) return;
    const limit = limitSelect.value;
    const terminal = document.getElementById('terminal');
    const statusText = document.getElementById('status-text');
    const statusPulse = document.getElementById('status-pulse');

    try {
        const response = await fetch(`/api/logs?limit=${limit}`);
        if (!response.ok) throw new Error("Erro na rede");
        const data = await response.json();

        const logsText = data.logs || "Nenhum log gravado até o momento.";

        // Evita redesenhar se o log for idêntico
        if (logsText === lastLogContent) {
            statusText.innerText = "Monitorando (sem novas linhas)";
            return;
        }
        lastLogContent = logsText;

        const lines = logsText.split("\n");
        let formattedHTML = "";
        for (let line of lines) {
            formattedHTML += formatLogLine(line);
        }

        terminal.innerHTML = formattedHTML || "Nenhum log gravado até o momento.";

        statusText.innerText = "Monitorando - Atualizado: " + new Date().toLocaleTimeString();
        statusPulse.className = "inline-block w-2.5 h-2.5 rounded-full bg-emerald-500 animate-pulse";

        // Rolar para o fim se selecionado
        const autoScrollCheck = document.getElementById('autoScrollCheck');
        if (autoScrollCheck && autoScrollCheck.checked) {
            terminal.scrollTop = terminal.scrollHeight;
        }
    } catch (err) {
        if (statusText) statusText.innerText = "Erro ao buscar logs do bot";
        if (statusPulse) statusPulse.className = "inline-block w-2.5 h-2.5 rounded-full bg-rose-500 animate-ping";
    }
}

function clearConsole() {
    document.getElementById('terminal').innerHTML = '<div class="text-zinc-650 italic font-mono">// Tela limpa localmente. Aguardando novas linhas...</div>';
    lastLogContent = "";
}

function setupAutoRefresh() {
    const intervalSelect = document.getElementById('intervalSelect');
    if (!intervalSelect) return;
    const interval = parseInt(intervalSelect.value);
    const statusPulse = document.getElementById('status-pulse');
    const statusText = document.getElementById('status-text');

    if (refreshInterval) {
        clearInterval(refreshInterval);
        refreshInterval = null;
    }

    if (interval > 0) {
        refreshInterval = setInterval(fetchLogs, interval);
        if (statusPulse) statusPulse.className = "inline-block w-2.5 h-2.5 rounded-full bg-emerald-500 animate-pulse";
        if (statusText) statusText.innerText = "Monitorando...";
    } else {
        if (statusPulse) statusPulse.className = "inline-block w-2.5 h-2.5 rounded-full bg-zinc-600";
        if (statusText) statusText.innerText = "Atualização automática desativada";
    }
}

window.addEventListener('DOMContentLoaded', () => {
    fetchLogs();
    setupAutoRefresh();
});
