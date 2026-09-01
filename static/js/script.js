let statusInterval;
let lastQR = "";
let lastStatus = "";

async function checkStatus() {
    try {
        const response = await fetch('/api/status');
        const data = await response.json();

        const qrStatusText = document.getElementById('qr-status');
        const qrCodeDiv = document.getElementById('qrcode');

        const statusText = document.getElementById('status-text');
        const statusDot = document.getElementById('status-dot');
        const infoTimezone = document.getElementById('info-timezone');
        const infoEngine = document.getElementById('info-engine');
        const infoUptime = document.getElementById('info-uptime');

        if (!qrStatusText || !qrCodeDiv) return;

        if (infoTimezone) infoTimezone.innerText = data.timezone || 'America/Sao_Paulo';
        if (infoEngine) infoEngine.innerText = data.engine || 'Whatsmeow';
        if (infoUptime) infoUptime.innerText = data.uptime || '0s';

        if (data.status !== lastStatus) {
            lastStatus = data.status;

            if (data.status === 'qr' && data.qr) {
                qrStatusText.style.display = 'none';

                if (statusText) {
                    statusText.innerText = "Aguardando QR Code";
                    statusText.className = "text-xs font-bold text-amber-400";
                    if (statusDot) statusDot.className = "inline-block rounded-full h-2 w-2 bg-amber-500";
                }

                lastQR = data.qr;
                const qrImageUrl = `https://api.qrserver.com/v1/create-qr-code/?size=220x220&data=${encodeURIComponent(data.qr)}`;
                qrCodeDiv.innerHTML = `
                    <div class="qr-container-wrapper flex flex-col items-center">
                        <img src="${qrImageUrl}" alt="QR Code do WhatsApp" class="rounded border border-zinc-800">
                        <div class="mt-3 flex gap-2 w-full justify-center">
                            <button onclick="whatsappConnect()" class="px-3 py-1.5 bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-zinc-100 rounded text-xs font-semibold transition">
                                Conectar
                            </button>
                            <button onclick="whatsappLogout()" class="px-3 py-1.5 bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 rounded text-xs font-semibold transition">
                                Resetar Sessão
                            </button>
                        </div>
                    </div>
                `;
            }
            else if (data.status === 'connected') {
                qrCodeDiv.innerHTML = `
                    <div class="flex flex-col items-center justify-center p-4 text-center">
                        <div class="h-12 w-12 bg-emerald-950/80 border border-emerald-800/80 rounded-full flex items-center justify-center mb-3">
                            <svg class="w-6 h-6 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
                        </div>
                        <span class="text-emerald-400 font-bold text-xs uppercase tracking-wider">Sessão Ativa</span>
                        <p class="text-[11px] text-zinc-400 mt-1 font-normal">O bot está pronto e operando no servidor</p>
                        <div class="mt-4 flex gap-2 w-full justify-center">
                            <a href="/chats" class="px-3 py-1.5 bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-zinc-100 rounded text-xs font-semibold transition">
                                Abrir Conversas
                            </a>
                            <button onclick="whatsappLogout()" class="px-3 py-1.5 bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 rounded text-xs font-semibold transition">
                                Desconectar
                            </button>
                        </div>
                    </div>
                `;
                qrStatusText.style.display = 'block';
                qrStatusText.innerHTML = `<span class="text-emerald-400 font-semibold text-xs">Bot operacional na rede</span>`;

                if (statusText) {
                    statusText.innerText = "Conectado";
                    statusText.className = "text-xs font-bold text-emerald-400";
                    if (statusDot) statusDot.className = "inline-block rounded-full h-2 w-2 bg-emerald-500";
                }
            }
            else if (data.status === 'waiting') {
                qrCodeDiv.innerHTML = `
                    <div class="flex flex-col items-center justify-center p-4">
                        <svg class="w-6 h-6 animate-spin mb-2 text-zinc-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
                        <div class="mt-3 flex gap-2">
                            <button onclick="whatsappConnect()" class="px-3 py-1.5 bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-zinc-100 rounded text-xs font-semibold transition">
                                Conectar
                            </button>
                            <button onclick="whatsappLogout()" class="px-3 py-1.5 bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 rounded text-xs font-semibold transition">
                                Resetar Sessão
                            </button>
                        </div>
                    </div>
                `;
                qrStatusText.style.display = 'block';
                qrStatusText.innerHTML = `<span class="text-zinc-400 font-semibold text-xs">Aguardando geração do QR Code...</span>`;

                if (statusText) {
                    statusText.innerText = "Inicializando...";
                    statusText.className = "text-xs font-bold text-amber-400";
                    if (statusDot) statusDot.className = "inline-block rounded-full h-2 w-2 bg-amber-500";
                }
            }
        } else {
            if (data.status === 'qr' && data.qr && data.qr !== lastQR) {
                lastQR = data.qr;
                const qrImageUrl = `https://api.qrserver.com/v1/create-qr-code/?size=220x220&data=${encodeURIComponent(data.qr)}`;
                qrCodeDiv.innerHTML = `
                    <div class="qr-container-wrapper flex flex-col items-center">
                        <img src="${qrImageUrl}" alt="QR Code do WhatsApp" class="rounded border border-zinc-800">
                        <div class="mt-3 flex gap-2 w-full justify-center">
                            <button onclick="whatsappConnect()" class="px-3 py-1.5 bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-zinc-100 rounded text-xs font-semibold transition">
                                Conectar
                            </button>
                            <button onclick="whatsappLogout()" class="px-3 py-1.5 bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 rounded text-xs font-semibold transition">
                                Resetar Sessão
                            </button>
                        </div>
                    </div>
                `;
            }
        }
    } catch (error) {
        lastStatus = "error";
        console.error("Erro ao buscar status:", error);

        const qrStatusText = document.getElementById('qr-status');
        const statusText = document.getElementById('status-text');
        const statusDot = document.getElementById('status-dot');
        const infoTimezone = document.getElementById('info-timezone');
        const infoEngine = document.getElementById('info-engine');
        const infoUptime = document.getElementById('info-uptime');

        if (qrStatusText) {
            qrStatusText.style.display = 'block';
            qrStatusText.innerHTML = `<span class="text-rose-400 font-semibold text-xs">Erro de conexão com o servidor</span>`;
        }

        if (statusText) {
            statusText.innerText = "Desconectado";
            statusText.className = "text-xs font-bold text-rose-400";
            if (statusDot) statusDot.className = "inline-block rounded-full h-2 w-2 bg-rose-500";
        }

        if (infoTimezone) infoTimezone.innerText = 'Indisponível';
        if (infoEngine) infoEngine.innerText = 'Desconectado';
        if (infoUptime) infoUptime.innerText = 'Offline';
    }
}

statusInterval = setInterval(checkStatus, 1000);
checkStatus();

async function fetchRecentTickets() {
    try {
        const response = await fetch('/api/tickets/recent');
        if (!response.ok) throw new Error("Erro ao buscar chamados recentes");
        const tickets = await response.json();

        const body = document.getElementById('recent-tickets-body');
        if (!body) return;

        if (tickets.length === 0) {
            body.innerHTML = `
                <tr>
                    <td colspan="4" class="py-3 px-3 text-center text-zinc-500 italic">Nenhum chamado pendente na fila.</td>
                </tr>
            `;
            return;
        }

        let html = "";
        tickets.forEach(t => {
            html += `
                <tr class="hover:bg-zinc-800/40 border-b border-zinc-800/60 transition">
                    <td class="py-2.5 px-3 font-mono text-zinc-100 font-bold">#${t.ticket_id}</td>
                    <td class="py-2.5 px-3 text-zinc-200 font-medium">${t.requester}</td>
                    <td class="py-2.5 px-3 text-zinc-200 max-w-xs truncate" title="${t.title}">${t.title}</td>
                    <td class="py-2.5 px-3 text-right text-zinc-400 font-mono text-[11px]">${t.created_at}</td>
                </tr>
            `;
        });
        body.innerHTML = html;
    } catch (error) {
        console.error("Erro ao buscar chamados recentes:", error);
    }
}

fetchRecentTickets();
setInterval(fetchRecentTickets, 10000);

async function whatsappConnect() {
    try {
        const res = await fetch('/api/whatsapp/connect', { method: 'POST' });
        const data = await res.json();
        if (res.ok) {
            checkStatus();
        } else {
            alert("Erro ao conectar: " + (data.error || res.statusText));
        }
    } catch (err) {
        console.error(err);
        alert("Erro de rede ao conectar o WhatsApp");
    }
}

async function whatsappLogout() {
    if (!confirm("Tem certeza que deseja desconectar o bot e limpar a sessão ativa? Isso irá gerar um novo QR Code.")) {
        return;
    }
    try {
        const res = await fetch('/api/whatsapp/logout', { method: 'POST' });
        const data = await res.json();
        if (res.ok) {
            checkStatus();
        } else {
            alert("Erro ao deslogar: " + (data.error || res.statusText));
        }
    } catch (err) {
        console.error(err);
        alert("Erro de rede ao deslogar o WhatsApp");
    }
}