/**
 * SCRIPT: script.js
 * Descrição: Lógica de controle de frontend para a interface administrativa do GLPI-BOT.
 */

let statusInterval;
let lastQR = "";
let lastStatus = "";

// Função checkStatus manipula a rotina correspondente na interface do painel
async function checkStatus() {
    try {
        const response = await fetch('/api/status');
        const data = await response.json();
        
        const qrStatusText = document.getElementById('qr-status');
        const qrCodeDiv = document.getElementById('qrcode');
        
        // Elementos de Diagnóstico
        const statusText = document.getElementById('status-text');
        const statusDot = document.getElementById('status-dot');
        const statusPing = document.querySelector('.animate-ping');
        const infoTimezone = document.getElementById('info-timezone');
        const infoEngine = document.getElementById('info-engine');
        const infoUptime = document.getElementById('info-uptime');

        // Proteção: Se o HTML ainda não carregou, não tenta atualizar
        if (!qrStatusText || !qrCodeDiv) return;

        // Atualiza os campos que mudam constantemente (sem piscar a tela)
        if (infoTimezone) infoTimezone.innerText = data.timezone || 'America/Sao_Paulo';
        if (infoEngine) infoEngine.innerText = data.engine || 'Whatsmeow';
        if (infoUptime) infoUptime.innerText = data.uptime || '0s';

        // Só atualiza os elementos principais se o status geral mudou
        if (data.status !== lastStatus) {
            lastStatus = data.status;
            
            if (data.status === 'qr' && data.qr) {
                // Status: Aguardando leitura do QR Code
                qrStatusText.style.display = 'none';
                
                if (statusText) {
                    statusText.innerText = "Aguardando QR Code";
                    statusText.className = "text-xs font-semibold text-yellow-400 font-bold";
                    statusDot.className = "relative inline-flex rounded-full h-2 w-2 bg-yellow-500";
                    if (statusPing) statusPing.className = "animate-ping absolute inline-flex h-full w-full rounded-full bg-yellow-400 opacity-75";
                }

                lastQR = data.qr;
                const qrImageUrl = `https://api.qrserver.com/v1/create-qr-code/?size=220x220&data=${encodeURIComponent(data.qr)}`;
                qrCodeDiv.innerHTML = `
                    <div class="qr-container-wrapper flex flex-col items-center">
                        <img src="${qrImageUrl}" alt="QR Code do WhatsApp" class="qr-animate">
                        <div class="mt-4 flex gap-2 w-full justify-center">
                            <button onclick="whatsappConnect()" class="px-4 py-2 bg-indigo-500/10 hover:bg-indigo-500/20 border border-indigo-500/30 hover:border-indigo-500/50 text-indigo-400 hover:text-indigo-300 rounded-xl text-xs font-semibold transition duration-150">
                                🔌 Conectar
                            </button>
                            <button onclick="whatsappLogout()" class="px-4 py-2 bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 hover:border-red-500/50 text-red-400 hover:text-red-300 rounded-xl text-xs font-semibold transition duration-150">
                                🧹 Resetar
                            </button>
                        </div>
                    </div>
                `;
            } 
            else if (data.status === 'connected') {
                // Status: Conectado com sucesso
                qrCodeDiv.innerHTML = `
                    <div class="flex flex-col items-center justify-center p-6 text-center">
                        <div class="h-16 w-16 bg-emerald-500/10 border border-emerald-500/30 rounded-full flex items-center justify-center mb-4 shadow-lg shadow-emerald-500/5 animate-pulse">
                            <span class="text-3xl text-emerald-400">✔️</span>
                        </div>
                        <span class="text-emerald-400 font-bold text-lg tracking-wide">Sessão Ativa</span>
                        <p class="text-xs text-gray-400 mt-1 font-normal">O bot está pronto e operando</p>
                        <div class="mt-5 flex gap-2 w-full justify-center">
                            <a href="/chats" class="px-4 py-2 bg-white/10 hover:bg-white/20 border border-white/20 hover:border-white/30 text-white rounded-xl text-xs font-semibold transition duration-150 flex items-center gap-1.5 shadow-md">
                                💬 Abrir Conversas
                            </a>
                            <button onclick="whatsappLogout()" class="px-4 py-2 bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 hover:border-red-500/50 text-red-400 hover:text-red-300 rounded-xl text-xs font-semibold transition duration-150 flex items-center gap-1.5 shadow-lg">
                                ❌ Desconectar
                            </button>
                        </div>
                    </div>
                `;
                qrStatusText.style.display = 'block';
                qrStatusText.innerHTML = `<span class="text-emerald-500 font-semibold text-xs">Bot operacional na rede</span>`;
                
                if (statusText) {
                    statusText.innerText = "Conectado";
                    statusText.className = "text-xs font-semibold text-emerald-400 font-bold";
                    statusDot.className = "relative inline-flex rounded-full h-2 w-2 bg-emerald-500";
                    if (statusPing) statusPing.className = "animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75";
                }
            }
            else if (data.status === 'waiting') {
                // Status: O servidor ligou, mas a biblioteca ainda não cuspiu o QR Code
                qrCodeDiv.innerHTML = `
                    <div class="flex flex-col items-center justify-center p-4">
                        <span class="text-4xl animate-spin mb-3 text-indigo-400">🔄</span>
                        <div class="mt-4 flex gap-2">
                            <button onclick="whatsappConnect()" class="px-4 py-2 bg-indigo-500/10 hover:bg-indigo-500/20 border border-indigo-500/30 hover:border-indigo-500/50 text-indigo-400 hover:text-indigo-300 rounded-xl text-xs font-semibold transition duration-150">
                                🔌 Conectar
                            </button>
                            <button onclick="whatsappLogout()" class="px-4 py-2 bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 hover:border-red-500/50 text-red-400 hover:text-red-300 rounded-xl text-xs font-semibold transition duration-150">
                                🧹 Resetar
                            </button>
                        </div>
                    </div>
                `;
                qrStatusText.style.display = 'block';
                qrStatusText.innerHTML = `<span class="text-gray-400 font-bold animate-pulse text-xs">Aguardando geração do QR Code...</span>`;

                if (statusText) {
                    statusText.innerText = "Inicializando...";
                    statusText.className = "text-xs font-semibold text-cyan-400 font-bold";
                    statusDot.className = "relative inline-flex rounded-full h-2 w-2 bg-cyan-500";
                    if (statusPing) statusPing.className = "animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75";
                }
            }
        } else {
            // Se o status continua sendo 'qr' mas o valor do token mudou, regera o QR Code
            if (data.status === 'qr' && data.qr && data.qr !== lastQR) {
                lastQR = data.qr;
                const qrImageUrl = `https://api.qrserver.com/v1/create-qr-code/?size=220x220&data=${encodeURIComponent(data.qr)}`;
                qrCodeDiv.innerHTML = `
                    <div class="qr-container-wrapper flex flex-col items-center">
                        <img src="${qrImageUrl}" alt="QR Code do WhatsApp" class="qr-animate">
                        <div class="mt-4 flex gap-2 w-full justify-center">
                            <button onclick="whatsappConnect()" class="px-4 py-2 bg-indigo-500/10 hover:bg-indigo-500/20 border border-indigo-500/30 hover:border-indigo-500/50 text-indigo-400 hover:text-indigo-300 rounded-xl text-xs font-semibold transition duration-150">
                                🔌 Conectar
                            </button>
                            <button onclick="whatsappLogout()" class="px-4 py-2 bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 hover:border-red-500/50 text-red-400 hover:text-red-300 rounded-xl text-xs font-semibold transition duration-150">
                                🧹 Resetar
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
        const statusPing = document.querySelector('.animate-ping');
        const infoTimezone = document.getElementById('info-timezone');
        const infoEngine = document.getElementById('info-engine');
        const infoUptime = document.getElementById('info-uptime');

        if (qrStatusText) {
            qrStatusText.style.display = 'block';
            qrStatusText.innerHTML = `<span class="text-red-500 font-bold text-xs">❌ Erro de conexão com a API</span>`;
        }

        if (statusText) {
            statusText.innerText = "Desconectado (Erro)";
            statusText.className = "text-xs font-semibold text-red-400 font-bold";
            statusDot.className = "relative inline-flex rounded-full h-2 w-2 bg-red-500";
            if (statusPing) statusPing.className = "animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75";
        }

        // Zera os diagnósticos em caso de erro na conexão
        if (infoTimezone) infoTimezone.innerText = 'Indisponível';
        if (infoEngine) infoEngine.innerText = 'Desconectado';
        if (infoUptime) infoUptime.innerText = 'Offline';
    }
}

// Inicia o loop para checar o status a cada 1 segundo (atualização de uplink fluida)
statusInterval = setInterval(checkStatus, 1000);

// Faz a primeira checagem imediatamente ao abrir a página
checkStatus();

// Histórico de chamados recentes
// Função fetchRecentTickets manipula a rotina correspondente na interface do painel
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
                    <td colspan="4" class="py-4 text-center text-zinc-500 italic">Nenhum chamado aberto recentemente.</td>
                </tr>
            `;
            return;
        }

        let html = "";
        tickets.forEach(t => {
            html += `
                <tr class="hover:bg-white/5 transition duration-150">
                    <td class="py-3.5 font-mono text-indigo-400 font-semibold">#${t.ticket_id}</td>
                    <td class="py-3.5 font-medium">${t.requester}</td>
                    <td class="py-3.5 max-w-xs truncate" title="${t.title}">${t.title}</td>
                    <td class="py-3.5 text-right text-zinc-500">${t.created_at}</td>
                </tr>
            `;
        });
        body.innerHTML = html;
    } catch (error) {
        console.error("Erro ao buscar chamados recentes:", error);
    }
}

// Busca os chamados recentes ao iniciar e a cada 10 segundos
fetchRecentTickets();
setInterval(fetchRecentTickets, 10000);

// Chamadas de controle de conexao do WhatsApp
// Função whatsappConnect manipula a rotina correspondente na interface do painel
async function whatsappConnect() {
    try {
        const res = await fetch('/api/whatsapp/connect', { method: 'POST' });
        const data = await res.json();
        if (res.ok) {
            console.log("Solitação de conexão enviada com sucesso.");
            checkStatus();
        } else {
            alert("Erro ao conectar: " + (data.error || res.statusText));
        }
    } catch (err) {
        console.error(err);
        alert("Erro de rede ao conectar o WhatsApp");
    }
}

// Função whatsappLogout manipula a rotina correspondente na interface do painel
async function whatsappLogout() {
    if (!confirm("Tem certeza que deseja desconectar o bot e limpar a sessão ativa? Isso irá gerar um novo QR Code.")) {
        return;
    }
    try {
        const res = await fetch('/api/whatsapp/logout', { method: 'POST' });
        const data = await res.json();
        if (res.ok) {
            console.log("Solicitação de logout/reset enviada com sucesso.");
            checkStatus();
        } else {
            alert("Erro ao deslogar: " + (data.error || res.statusText));
        }
    } catch (err) {
        console.error(err);
        alert("Erro de rede ao deslogar o WhatsApp");
    }
}