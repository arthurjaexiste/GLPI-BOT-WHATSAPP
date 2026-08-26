/**
 * SCRIPT: chats.js
 * Descrição: Lógica de controle de frontend para a interface administrativa do GLPI-BOT.
 */

let activeChatJID = null;
let activeChatName = "";
let activeChatStatus = null;
let agentsList = [];
let chatsData = [];

// Formata JID para exibir o número do telefone de forma legível

// Função formatJIDToPhone manipula a rotina correspondente na interface do painel
function formatJIDToPhone(jid) {
    if (!jid) return "";
    // Remove sufixos como @s.whatsapp.net ou grupos, e também session id :1, :2
    const num = jid.split('@')[0].split(':')[0];

    // Formatação de número brasileiro (+55 DD 9XXXX-XXXX ou +55 DD XXXX-XXXX)
    if (num.startsWith('55') && num.length >= 10) {
        const ddd = num.substring(2, 4);
        const rest = num.substring(4);
        if (rest.length === 9) {
            return `+55 (${ddd}) ${rest.substring(0, 5)}-${rest.substring(5)}`;
        } else if (rest.length === 8) {
            return `+55 (${ddd}) ${rest.substring(0, 4)}-${rest.substring(4)}`;
        }
    }
    return `+${num}`;
}

// Ao carregar a página
document.addEventListener("DOMContentLoaded", () => {
    loadChatsList();

    // Inicia polling periódico em alta frequência para mensagens em tempo real
    setInterval(() => {
        if (activeChatJID) {
            refreshActiveMessages();
        }
    }, 1500);

    setInterval(() => {
        loadChatsList(true); // silent load
    }, 2000);
});

// Busca a lista de chats e exibe na barra lateral
// Função loadChatsList manipula a rotina correspondente na interface do painel
async function loadChatsList(silent = false) {
    try {
        const response = await fetch("/api/chats");
        if (!response.ok) throw new Error("Erro ao buscar conversas");

        chatsData = await response.json();
        renderChatsList();
    } catch (error) {
        console.error("Erro:", error);
        if (!silent) {
            const container = document.getElementById("chats-list-container");
            container.innerHTML = `<div class="text-center py-8 text-rose-400 text-xs italic">Erro ao carregar conversas.</div>`;
        }
    }
}

// Renderiza a lista de chats baseando-se no chatsData (e permite busca/filtro)

// Função renderChatsList manipula a rotina correspondente na interface do painel
function renderChatsList(filteredData = null) {
    const container = document.getElementById("chats-list-container");
    const data = filteredData || chatsData;

    if (data.length === 0) {
        container.innerHTML = `<div class="text-center py-8 text-zinc-500 text-xs italic">Nenhuma conversa encontrada.</div>`;
        return;
    }

    container.innerHTML = "";
    data.forEach(chat => {
        // Status badge config
        let statusText = "Bot";
        let statusClass = "status-bot";
        if (chat.status === "live_chat") {
            statusText = "Suporte";
            statusClass = "status-live_chat";
        } else if (chat.status === "queue") {
            statusText = "Fila";
            statusClass = "status-queue";
        }

        const isSelected = activeChatJID === chat.jid;
        const cardClass = isSelected
            ? "bg-white/10 border-white/10"
            : "bg-zinc-900/40 hover:bg-white/5 border-white/5";

        const card = document.createElement("div");
        card.className = `p-3 rounded-xl border cursor-pointer transition flex flex-col gap-1.5 relative group ${cardClass}`;
        card.onclick = () => selectChat(chat.jid, chat.name, chat.status);

        // Limita o tamanho do texto da última mensagem
        let snippet = chat.last_message || "Nenhuma mensagem...";
        if (snippet.length > 35) snippet = snippet.substring(0, 35) + "...";

        const phoneFormatted = formatJIDToPhone(chat.jid);
        const displayName = chat.name ? chat.name : phoneFormatted;
        const subText = chat.name ? `<span class="text-[9px] text-zinc-400 font-mono">${phoneFormatted}</span>` : '';

        card.innerHTML = `
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-2.5 min-w-0">
                    <img src="/api/chats/avatar?jid=${encodeURIComponent(chat.jid)}&name=${encodeURIComponent(displayName)}" class="w-8 h-8 rounded-lg object-cover border border-white/10 flex-shrink-0" onerror="this.onerror=null; this.src='https://ui-avatars.com/api/?name=${encodeURIComponent(displayName)}&background=random&color=fff';" />
                    <div class="flex flex-col min-w-0">
                        <span class="text-xs font-bold text-zinc-200 truncate max-w-[125px]">${displayName}</span>
                        ${subText}
                    </div>
                </div>
                <div class="flex items-center gap-1.5 flex-shrink-0">
                    <span class="status-dot ${statusClass} group-hover:hidden"></span>
                    <span class="text-[9px] font-bold text-zinc-500 uppercase tracking-widest group-hover:hidden">${statusText}</span>
                    <button onclick="event.stopPropagation(); deleteChat('${chat.jid}');" class="hidden group-hover:inline-flex items-center justify-center text-rose-400 hover:text-rose-300 transition text-xs p-1 hover:bg-white/5 rounded-md" title="Apagar conversa">
                        🗑️
                    </button>
                </div>
            </div>
            <div class="flex items-center justify-between gap-2 mt-1">
                <span class="text-[10px] text-zinc-400 truncate flex-1">${snippet}</span>
                <span class="text-[9px] text-zinc-500 font-medium">${chat.timestamp}</span>
            </div>
        `;
        container.appendChild(card);
    });
}

// Filtra os chats conforme digitação no search

// Função filterChats manipula a rotina correspondente na interface do painel
function filterChats() {
    const term = document.getElementById("chat-search-input").value.toLowerCase();
    if (!term) {
        renderChatsList();
        return;
    }
    const filtered = chatsData.filter(chat => {
        return (chat.name && chat.name.toLowerCase().includes(term)) ||
            chat.jid.toLowerCase().includes(term) ||
            (chat.last_message && chat.last_message.toLowerCase().includes(term));
    });
    renderChatsList(filtered);
}

// Seleciona um chat da lista
// Função selectChat manipula a rotina correspondente na interface do painel
async function selectChat(jid, name, status) {
    activeChatJID = jid;
    activeChatName = name || jid;
    activeChatStatus = status;

    // Esconde o placeholder
    document.getElementById("chat-placeholder").style.display = "none";

    // Configura o cabeçalho do chat ativo
    const phoneFormatted = formatJIDToPhone(jid);
    const displayName = name ? name : phoneFormatted;

    document.getElementById("active-chat-name").textContent = displayName;
    document.getElementById("active-chat-jid").textContent = phoneFormatted;

    // Configura o avatar no cabeçalho
    const avatarContainer = document.getElementById("active-chat-avatar");
    avatarContainer.innerHTML = `<img src="/api/chats/avatar?jid=${encodeURIComponent(jid)}&name=${encodeURIComponent(displayName)}" class="w-full h-full rounded-xl object-cover" onerror="this.onerror=null; this.src='https://ui-avatars.com/api/?name=${encodeURIComponent(displayName)}&background=random&color=fff';" />`;

    const statusDot = document.getElementById("active-chat-status-dot");
    const statusText = document.getElementById("active-chat-status-text");

    statusDot.className = "w-1.5 h-1.5 rounded-full";
    if (status === "live_chat") {
        statusDot.classList.add("status-live_chat");
        statusText.textContent = "Live Chat / Suporte";
        document.getElementById("assume-chat-banner").style.display = "none";
    } else if (status === "queue") {
        statusDot.classList.add("status-queue");
        statusText.textContent = "Fila de Espera";
        document.getElementById("assume-chat-banner").style.display = "flex";
    } else {
        statusDot.classList.add("status-bot");
        statusText.textContent = "Interação Bot";
        document.getElementById("assume-chat-banner").style.display = "flex";
    }

    // Limpa a tela
    document.getElementById("chats-list-container").childNodes.forEach(node => {
        node.classList.remove("bg-white/10", "border-white/10");
        node.classList.add("bg-zinc-900/40", "border-white/5");
    });

    // Atualiza a lista lateral para destacar a selecionada
    loadChatsList(true);

    // Carrega histórico de mensagens
    await refreshActiveMessages(true);
}

// Atualiza o histórico de mensagens
let lastMessagesCount = 0;
// Função refreshActiveMessages manipula a rotina correspondente na interface do painel
async function refreshActiveMessages(forceScroll = false) {
    if (!activeChatJID) return;

    try {
        const response = await fetch(`/api/chats/messages?jid=${encodeURIComponent(activeChatJID)}`);
        if (!response.ok) throw new Error("Erro ao buscar histórico");

        const messages = await response.json();
        renderMessages(messages, forceScroll);
    } catch (error) {
        console.error("Erro de sincronização de mensagens:", error);
    }
}

let activeReplyQuote = null;
let selectedMediaFile = null;

function handleFileSelected(event) {
    const files = event.target.files;
    if (!files || files.length === 0) return;

    selectedMediaFile = files[0];
    const preview = document.getElementById("media-attachment-preview");
    const imgEl = document.getElementById("media-preview-img");
    const iconEl = document.getElementById("media-preview-icon");
    const nameEl = document.getElementById("media-preview-filename");
    const sizeEl = document.getElementById("media-preview-size");

    if (nameEl) nameEl.textContent = selectedMediaFile.name;
    if (sizeEl) {
        const kb = selectedMediaFile.size / 1024;
        sizeEl.textContent = kb > 1024 ? (kb / 1024).toFixed(1) + " MB" : Math.round(kb) + " KB";
    }

    if (selectedMediaFile.type.startsWith("image/")) {
        const reader = new FileReader();
        reader.onload = function (e) {
            if (imgEl) {
                imgEl.src = e.target.result;
                imgEl.classList.remove("hidden");
            }
            if (iconEl) iconEl.classList.add("hidden");
        };
        reader.readAsDataURL(selectedMediaFile);
    } else {
        if (imgEl) imgEl.classList.add("hidden");
        if (iconEl) iconEl.classList.remove("hidden");
    }

    if (preview) preview.classList.remove("hidden");
    const input = document.getElementById("chat-message-input");
    if (input) input.focus();
}

function clearSelectedMedia() {
    selectedMediaFile = null;
    const fileInput = document.getElementById("chat-file-input");
    if (fileInput) fileInput.value = "";
    const preview = document.getElementById("media-attachment-preview");
    if (preview) preview.classList.add("hidden");
    const imgEl = document.getElementById("media-preview-img");
    if (imgEl) imgEl.src = "";
}

// Suporte para colar imagem da área de transferência (Ctrl+V)
document.addEventListener("paste", function (e) {
    if (!e.clipboardData || !e.clipboardData.items) return;
    const items = e.clipboardData.items;
    for (let i = 0; i < items.length; i++) {
        if (items[i].type.indexOf("image") !== -1) {
            const file = items[i].getAsFile();
            if (file) {
                const dt = new DataTransfer();
                dt.items.add(file);
                const fileInput = document.getElementById("chat-file-input");
                if (fileInput) {
                    fileInput.files = dt.files;
                    handleFileSelected({ target: fileInput });
                }
            }
            break;
        }
    }
});

// ─── Gravação de Áudio de Voz (Microfone) ───────────────────────────────────
let mediaRecorder = null;
let audioChunks = [];
let recordingTimerInterval = null;
let recordingSeconds = 0;

async function toggleAudioRecording() {
    if (mediaRecorder && mediaRecorder.state === "recording") {
        stopAndSendAudioRecording();
        return;
    }

    try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        audioChunks = [];
        mediaRecorder = new MediaRecorder(stream);

        mediaRecorder.ondataavailable = function (e) {
            if (e.data.size > 0) audioChunks.push(e.data);
        };

        mediaRecorder.onstop = async function () {
            stream.getTracks().forEach(track => track.stop());
            clearInterval(recordingTimerInterval);
            const recBar = document.getElementById("audio-recording-bar");
            if (recBar) recBar.classList.add("hidden");

            if (audioChunks.length > 0) {
                const audioBlob = new Blob(audioChunks, { type: "audio/ogg; codecs=opus" });
                const audioFile = new File([audioBlob], `audio_recorded_${Date.now()}.ogg`, { type: "audio/ogg" });
                
                selectedMediaFile = audioFile;
                await sendConsoleMessage();
            }
        };

        mediaRecorder.start();
        recordingSeconds = 0;
        const timerEl = document.getElementById("recording-timer");
        if (timerEl) timerEl.textContent = "00:00";
        const recBar = document.getElementById("audio-recording-bar");
        if (recBar) recBar.classList.remove("hidden");

        recordingTimerInterval = setInterval(() => {
            recordingSeconds++;
            const mins = String(Math.floor(recordingSeconds / 60)).padStart(2, '0');
            const secs = String(recordingSeconds % 60).padStart(2, '0');
            if (timerEl) timerEl.textContent = `${mins}:${secs}`;
        }, 1000);

    } catch (err) {
        console.error(err);
        showToast("Permissão de microfone negada ou indisponível.", false);
    }
}

function cancelAudioRecording() {
    if (mediaRecorder && mediaRecorder.state === "recording") {
        audioChunks = []; // Esvazia para não enviar ao parar
        mediaRecorder.stop();
    }
    clearInterval(recordingTimerInterval);
    const recBar = document.getElementById("audio-recording-bar");
    if (recBar) recBar.classList.add("hidden");
}

function stopAndSendAudioRecording() {
    if (mediaRecorder && mediaRecorder.state === "recording") {
        mediaRecorder.stop();
    }
}

function replyToMessage(senderName, originalText, waMsgId = '', senderJid = '') {
    let cleanName = senderName;
    if (!cleanName || cleanName === "Contato" || cleanName.startsWith("17188") || (cleanName.length >= 10 && !isNaN(cleanName))) {
        cleanName = activeChatName || formatJIDToPhone(activeChatJID);
    }
    activeReplyQuote = {
        name: cleanName,
        text: originalText || "",
        quoted_id: waMsgId || "",
        quoted_jid: senderJid || ""
    };

    const preview = document.getElementById("reply-quote-preview");
    const nameEl = document.getElementById("reply-quote-name");
    const textEl = document.getElementById("reply-quote-text");
    const input = document.getElementById("chat-message-input");

    if (preview && nameEl && textEl) {
        nameEl.textContent = `Respondendo a ${activeReplyQuote.name}`;
        let snippet = activeReplyQuote.text;
        if (snippet.length > 60) snippet = snippet.substring(0, 60) + "...";
        textEl.textContent = snippet || "Mensagem em mídia";
        preview.classList.remove("hidden");
    }

    if (input) input.focus();
}

function cancelReplyQuote() {
    activeReplyQuote = null;
    const preview = document.getElementById("reply-quote-preview");
    if (preview) preview.classList.add("hidden");
}

function escapeHTML(str) {
    if (!str) return "";
    return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}

// Renderiza o histórico de mensagens

// Função renderMessages manipula a rotina correspondente na interface do painel
function renderMessages(messages, forceScroll = false) {
    const container = document.getElementById("chat-messages-container");

    // Se não há novas mensagens, evita re-renderizar para manter o scroll amigável do admin
    if (messages.length === lastMessagesCount && !forceScroll) {
        return;
    }

    container.innerHTML = "";
    lastMessagesCount = messages.length;

    if (messages.length === 0) {
        container.innerHTML = `<div class="text-center py-8 text-zinc-500 text-xs italic">Nenhuma mensagem nesta conversa.</div>`;
        return;
    }

    messages.forEach(msg => {
        const row = document.createElement("div");
        row.className = `w-full flex ${msg.is_from_me ? 'justify-end' : 'justify-start'} items-center gap-2 my-1 group`;

        const bubble = document.createElement("div");
        bubble.className = `message-bubble ${msg.is_from_me ? 'message-outgoing' : 'message-incoming'}`;

        // Renderização de Mensagem Citada (WhatsApp Quote Box)
        let quoteHTML = "";
        if (msg.reply_to_text) {
            let authorName = msg.reply_to_name || 'Contato';
            if (!authorName || authorName === 'Contato' || authorName.startsWith("17188") || (authorName.length >= 10 && !isNaN(authorName))) {
                authorName = activeChatName || formatJIDToPhone(activeChatJID);
            }
            quoteHTML = `
                <div class="whatsapp-quote-box">
                    <div class="whatsapp-quote-author">${escapeHTML(authorName)}</div>
                    <div class="whatsapp-quote-text truncate">${escapeHTML(msg.reply_to_text)}</div>
                </div>
            `;
        }

        let mediaHTML = "";
        let textContent = msg.text || '';

        if (msg.media_url) {
            const lowerUrl = msg.media_url.toLowerCase();
            const isBase64Img = lowerUrl.startsWith("data:image/");
            const isBase64Audio = lowerUrl.startsWith("data:audio/");
            const isBase64Doc = lowerUrl.startsWith("data:application/") || lowerUrl.startsWith("data:text/");

            if (msg.type === "image" || isBase64Img || lowerUrl.endsWith(".jpg") || lowerUrl.endsWith(".jpeg") || lowerUrl.endsWith(".png") || lowerUrl.endsWith(".webp") || lowerUrl.endsWith(".gif")) {
                mediaHTML = `<div class="mb-2 overflow-hidden rounded-xl">
                    <img src="${msg.media_url}" alt="Imagem do WhatsApp" class="w-full max-h-80 object-cover rounded-xl border border-white/10 shadow-lg cursor-pointer hover:opacity-90 transition" onclick="window.open('${msg.media_url}', '_blank')" title="Clique para expandir em nova aba" />
                </div>`;
            } else if (msg.type === "audio" || isBase64Audio || lowerUrl.endsWith(".ogg") || lowerUrl.endsWith(".mp3") || lowerUrl.endsWith(".m4a") || lowerUrl.endsWith(".wav") || lowerUrl.endsWith(".webm")) {
                mediaHTML = `<div class="mb-1 py-1">
                    <audio controls src="${msg.media_url}" class="max-w-[250px] h-9 accent-emerald-500 rounded-lg focus:outline-none"></audio>
                </div>`;
            } else if (msg.type === "document" || isBase64Doc || lowerUrl.endsWith(".pdf") || lowerUrl.endsWith(".doc") || lowerUrl.endsWith(".docx") || lowerUrl.endsWith(".txt") || lowerUrl.endsWith(".zip")) {
                mediaHTML = `<div class="mb-2">
                    <a href="${msg.media_url}" download="documento" target="_blank" class="inline-flex items-center gap-2 px-3 py-2 bg-white/10 hover:bg-white/20 border border-white/10 rounded-xl text-amber-400 font-mono text-xs transition">
                        📄 Baixar / Abrir Documento Anexo
                    </a>
                </div>`;
            }
            if (textContent.startsWith("[Imagem]") || textContent.startsWith("[Áudio]") || textContent.startsWith("[Documento] audio_") || textContent.startsWith("[Documento] voice_")) {
                if (msg.type === "audio" || isBase64Audio || lowerUrl.endsWith(".ogg") || lowerUrl.endsWith(".mp3") || lowerUrl.endsWith(".m4a") || lowerUrl.endsWith(".wav") || lowerUrl.endsWith(".webm")) {
                    textContent = "";
                } else {
                    textContent = textContent.replace("[Imagem]", "").replace("[Áudio]", "").trim();
                }
            }
        } else if (msg.type === "image" || textContent.includes("[Imagem]")) {
            mediaHTML = `<div class="mb-1 flex items-center gap-2 text-xs font-semibold text-amber-300 bg-amber-500/10 px-3 py-1.5 rounded-xl border border-amber-500/20">
                <span class="text-sm">📷</span>
                <span>Imagem enviada no WhatsApp</span>
            </div>`;
            if (textContent === "[Imagem]") {
                textContent = "";
            } else {
                textContent = textContent.replace(/\[Imagem\]/g, "").trim();
            }
        } else if (msg.type === "audio" || textContent.includes("[Áudio]")) {
            mediaHTML = `<div class="mb-1 flex items-center gap-2 text-xs font-semibold text-emerald-300 bg-emerald-500/10 px-3 py-1.5 rounded-xl border border-emerald-500/20">
                <span class="text-sm">🎙️</span>
                <span>Áudio de Voz do WhatsApp</span>
            </div>`;
            if (textContent === "[Áudio]") {
                textContent = "";
            }
        }

        const formattedText = textContent ? textContent.replace(/\n/g, "<br>") : "";

        const bubbleHTML = `
            ${quoteHTML}
            ${mediaHTML}
            ${formattedText ? `<div class="font-medium">${formattedText}</div>` : ''}
            <div class="text-[9px] mt-1 text-right ${msg.is_from_me ? 'text-zinc-500' : 'text-zinc-400'}">${msg.timestamp}</div>
        `;

        bubble.innerHTML = bubbleHTML;

        // Botão "Responder" rápido no hover
        const replyBtn = !msg.is_from_me ? `
            <button onclick="replyToMessage('${escapeQuotes(msg.sender_name || activeChatName)}', '${escapeQuotes(msg.text)}', '${escapeQuotes(msg.wa_message_id || '')}', '${escapeQuotes(msg.sender_jid || '')}')" class="opacity-0 group-hover:opacity-100 transition-opacity text-[10px] text-zinc-400 hover:text-white bg-white/5 hover:bg-white/10 px-2.5 py-1 rounded-lg shrink-0 border border-white/5 cursor-pointer" title="Responder esta mensagem">
                ↩️ Responder
            </button>
        ` : '';

        if (!msg.is_from_me) {
            row.appendChild(bubble);
            const btnSpan = document.createElement("span");
            btnSpan.innerHTML = replyBtn;
            row.appendChild(btnSpan);
        } else {
            row.appendChild(bubble);
        }

        container.appendChild(row);
    });

    // Rola para a base das mensagens
    container.scrollTop = container.scrollHeight;
}

function escapeQuotes(str) {
    if (!str) return "";
    return str.replace(/'/g, "\\'").replace(/"/g, "&quot;").replace(/\n/g, " ");
}

// Envia mensagem via Painel Console
// Função sendConsoleMessage manipula a rotina correspondente na interface do painel
async function sendConsoleMessage() {
    const input = document.getElementById("chat-message-input");
    const text = input.value.trim();

    if (!text && !selectedMediaFile) return;
    if (!activeChatJID) return;

    const sendBtn = document.getElementById("btn-send-msg");
    sendBtn.disabled = true;
    sendBtn.textContent = "Enviando...";

    try {
        let response;
        if (selectedMediaFile) {
            const formData = new FormData();
            formData.append("file", selectedMediaFile);
            formData.append("jid", activeChatJID);
            if (text) formData.append("caption", text);
            if (activeReplyQuote) {
                formData.append("reply_to_name", activeReplyQuote.name);
                formData.append("reply_to_text", activeReplyQuote.text);
                if (activeReplyQuote.quoted_id) formData.append("quoted_id", activeReplyQuote.quoted_id);
                if (activeReplyQuote.quoted_jid) formData.append("quoted_jid", activeReplyQuote.quoted_jid);
            }

            response = await fetch("/api/chats/send-media", {
                method: "POST",
                body: formData
            });
        } else {
            const payload = {
                jid: activeChatJID,
                text: text
            };

            if (activeReplyQuote) {
                payload.reply_to_name = activeReplyQuote.name;
                payload.reply_to_text = activeReplyQuote.text;
                payload.quoted_id = activeReplyQuote.quoted_id;
                payload.quoted_jid = activeReplyQuote.quoted_jid;
            }

            response = await fetch("/api/chats/send", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json"
                },
                body: JSON.stringify(payload)
            });
        }

        if (!response.ok) {
            const errText = await response.text();
            throw new Error(errText || "Falha ao enviar mensagem");
        }

        input.value = "";
        clearSelectedMedia();
        cancelReplyQuote();
        
        // Atualiza o estado da UI para Live Chat se estiver em bot ou fila
        if (activeChatStatus !== "live_chat") {
            activeChatStatus = "live_chat";
            const statusDot = document.getElementById("active-chat-status-dot");
            const statusText = document.getElementById("active-chat-status-text");
            if (statusDot && statusText) {
                statusDot.className = "w-1.5 h-1.5 rounded-full status-live_chat";
                statusText.textContent = "Live Chat / Suporte";
            }
            const banner = document.getElementById("assume-chat-banner");
            if (banner) banner.style.display = "none";
        }

        showToast("Enviado com sucesso!", true);
        await refreshActiveMessages(true);
        loadChatsList(true); // Atualiza snippet lateral
    } catch (error) {
        console.error(error);
        showToast(error.message || "Falha ao enviar mensagem.", false);
    } finally {
        sendBtn.disabled = false;
        sendBtn.textContent = "Enviar";
    }
}

// Detecta Enter no input

// Função handleInputKey manipula a rotina correspondente na interface do painel
function handleInputKey(e) {
    if (e.key === "Enter") {
        sendConsoleMessage();
    }
}

// Exibe Toast de notificação

// Função showToast manipula a rotina correspondente na interface do painel
function showToast(message, isSuccess = true) {
    const toast = document.getElementById("toast");
    const icon = document.getElementById("toast-icon");
    const msg = document.getElementById("toast-message");

    icon.textContent = isSuccess ? "✅" : "🚨";
    msg.textContent = message;

    toast.className = `fixed bottom-6 right-6 px-5 py-3 rounded-lg shadow-2xl backdrop-blur-md transform transition duration-300 flex items-center gap-3 z-50 ${isSuccess
            ? 'bg-emerald-950/80 border border-emerald-500/30 text-emerald-300'
            : 'bg-rose-950/80 border border-rose-500/30 text-rose-300'
        }`;

    toast.classList.remove("translate-y-24", "opacity-0");

    setTimeout(() => {
        toast.classList.add("translate-y-24", "opacity-0");
    }, 3000);
}

// Limpa completamente o estado da janela de chat e reseta para a tela inicial
function clearActiveChatUI() {
    activeChatJID = null;
    activeChatName = "";
    activeChatStatus = null;
    lastMessagesCount = 0;

    const placeholder = document.getElementById("chat-placeholder");
    const container = document.getElementById("chat-messages-container");
    const headerName = document.getElementById("active-chat-name");
    const headerJID = document.getElementById("active-chat-jid");
    const avatarContainer = document.getElementById("active-chat-avatar");
    const assumeBanner = document.getElementById("assume-chat-banner");

    if (placeholder) placeholder.style.display = "flex";
    if (container) container.innerHTML = "";
    if (headerName) headerName.textContent = "";
    if (headerJID) headerJID.textContent = "";
    if (avatarContainer) avatarContainer.innerHTML = "👤";
    if (assumeBanner) assumeBanner.style.display = "none";

    clearSelectedMedia();
    cancelReplyQuote();
}

// Apaga uma conversa específica
async function deleteChat(jid) {
    if (!confirm("Tem certeza que deseja apagar esta conversa e todo o histórico?")) {
        return;
    }

    try {
        const response = await fetch(`/api/chats/delete?jid=${encodeURIComponent(jid)}`, {
            method: "DELETE"
        });

        if (!response.ok) throw new Error("Erro ao apagar conversa");

        showToast("Conversa apagada com sucesso!");

        // Se a conversa apagada for a atualmente ativa ou tiver o mesmo número, reseta a interface
        const deletedNum = formatJIDToPhone(jid);
        const activeNum = activeChatJID ? formatJIDToPhone(activeChatJID) : "";

        if (!activeChatJID || activeChatJID === jid || deletedNum === activeNum) {
            clearActiveChatUI();
        }

        // Recarrega a lista de chats
        await loadChatsList();
    } catch (error) {
        console.error(error);
        showToast("Falha ao apagar conversa.", false);
    }
}

// Apaga a conversa ativa atualmente
// Função deleteActiveChat manipula a rotina correspondente na interface do painel
async function deleteActiveChat() {
    if (!activeChatJID) return;
    await deleteChat(activeChatJID);
}

// Finaliza o atendimento ativo atualmente
// Função closeActiveChat manipula a rotina correspondente na interface do painel
async function closeActiveChat() {
    if (!activeChatJID) return;
    await closeChat(activeChatJID);
}

// Finaliza o atendimento (devolve para o Bot)
// Função closeChat manipula a rotina correspondente na interface do painel
async function closeChat(jid) {
    if (!confirm("Deseja finalizar o atendimento humano e reativar o Bot para esta conversa?")) {
        return;
    }

    try {
        const response = await fetch(`/api/chats/close?jid=${encodeURIComponent(jid)}`, {
            method: "POST"
        });

        if (!response.ok) throw new Error("Erro ao finalizar atendimento");

        showToast("Atendimento finalizado. Bot reativado!");
        activeChatStatus = "bot";
        document.getElementById("assume-chat-banner").style.display = "flex";

        // Recarrega a lista de chats para atualizar o status visual
        loadChatsList();

        // Atualiza cabeçalho do chat ativo para mostrar "Interação Bot"
        const statusDot = document.getElementById("active-chat-status-dot");
        const statusText = document.getElementById("active-chat-status-text");
        if (statusDot && statusText) {
            statusDot.className = "w-1.5 h-1.5 rounded-full status-bot";
            statusText.textContent = "Interação Bot";
        }
    } catch (error) {
        console.error(error);
        showToast("Falha ao finalizar atendimento.", false);
    }
}

// Assume o chat ativo para o técnico logado na sessão
async function assumeActiveChat() {
    if (!activeChatJID) return;

    try {
        const response = await fetch("/api/chats/assume", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify({
                jid: activeChatJID
            })
        });

        if (!response.ok) throw new Error("Erro ao assumir atendimento");

        showToast("Atendimento assumido com sucesso!");
        document.getElementById("assume-chat-banner").style.display = "none";

        // Atualiza o status localmente para live_chat
        activeChatStatus = "live_chat";
        const statusDot = document.getElementById("active-chat-status-dot");
        const statusText = document.getElementById("active-chat-status-text");
        if (statusDot && statusText) {
            statusDot.className = "w-1.5 h-1.5 rounded-full status-live_chat";
            statusText.textContent = "Live Chat / Suporte";
        }

        // Recarrega a lista de chats para atualizar as tags
        await loadChatsList();
        refreshActiveMessages(true);
    } catch (error) {
        console.error(error);
        showToast("Falha ao assumir atendimento.", false);
    }
}
