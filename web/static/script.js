document.addEventListener('DOMContentLoaded', () => {
    const app = new KubeOwlApp();
    app.init();
});

class KubeOwlApp {
    constructor() {
        // Estado da aplicação
        this.currentSort = { key: 'name', order: 'asc' };
        this.selectedNamespace = ''; // Começa com Todos
        this.activeSection = 'dashboard';
        
        // Cache de dados completo
        this.fullDataCache = {
            pods: [], services: [], ingresses: [], pvcs: [], events: [],
            overview: {}, nodes: [], namespaces: []
        };
        
        this.ws = null;
        this.domElementMap = new Map(); // Mapeia UID para elemento do DOM para atualizações rápidas
        this.debounceTimer = null; // Para debounce de fetch
    }

    init() {
        this.setupTheme();
        this.setupNavigation();
        this.setupNamespaceSelector();
        this.fetchAllData(); // Busca todos os dados na inicialização
        this.setupWebSocket();
        
        // Atualiza apenas as métricas de nós e pods (que não vêm via watch) periodicamente
        setInterval(() => this.fetchMetrics(), 30000);
    }
    
    // --- Funções de Configuração Inicial ---

    setupTheme() {
        const themeToggle = document.getElementById('theme-toggle');
        const sunIcon = '<i class="fas fa-sun"></i>';
        const moonIcon = '<i class="fas fa-moon"></i>';

        const applyTheme = (theme) => {
            if (theme === 'dark') {
                document.documentElement.classList.add('dark');
                themeToggle.innerHTML = sunIcon;
            } else {
                document.documentElement.classList.remove('dark');
                themeToggle.innerHTML = moonIcon;
            }
        };

        const storedTheme = localStorage.getItem('theme');
        const preferredTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        const currentTheme = storedTheme || preferredTheme;
        applyTheme(currentTheme);

        themeToggle.addEventListener('click', () => {
            const isDark = document.documentElement.classList.toggle('dark');
            const newTheme = isDark ? 'dark' : 'light';
            localStorage.setItem('theme', newTheme);
            applyTheme(newTheme);
        });
    }
    
    setupNavigation() {
        const navLinks = document.querySelectorAll('.nav-link');
        const sections = document.querySelectorAll('.main-section');
        const sectionTitle = document.getElementById('section-title');
        const namespaceSelectorContainer = document.querySelector('.namespace-selector-container');

        navLinks.forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                this.activeSection = link.getAttribute('href').substring(1);
                const linkText = link.textContent.trim();

                navLinks.forEach(l => l.classList.remove('active'));
                link.classList.add('active');

                sections.forEach(s => {
                    s.classList.toggle('hidden', s.id !== `${this.activeSection}-section`);
                });
                
                sectionTitle.textContent = this.activeSection === 'dashboard' ? 'Visão Geral do Cluster' : linkText;
                
                const isDataSection = ['pods', 'services', 'ingresses', 'storage', 'events'].includes(this.activeSection);
                namespaceSelectorContainer.style.display = isDataSection ? 'flex' : 'none';

                this.renderActiveSection(); // Renderiza apenas a seção ativa
            });
        });
    }

    async setupNamespaceSelector() {
        const selector = document.getElementById('namespace-selector');
        selector.addEventListener('change', (e) => {
            this.selectedNamespace = e.target.value;
            this.renderActiveSection(); // Apenas re-renderiza com os dados do cache, sem fetch
        });
    }

    setupWebSocket() {
        const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(`${wsProtocol}//${window.location.host}/ws`);

        this.ws.onopen = () => this.updateConnectionStatus(true);
        this.ws.onmessage = (event) => this.handleWebSocketMessage(JSON.parse(event.data));
        this.ws.onclose = () => {
            this.updateConnectionStatus(false);
            setTimeout(() => this.setupWebSocket(), 5000);
        };
        this.ws.onerror = (error) => {
            console.error('Erro no WebSocket:', error);
            this.ws.close();
        };
    }

    // --- Funções de Busca de Dados (Otimizadas) ---

    async fetchAllData() {
        const endpoints = ['overview', 'nodes', 'namespaces', 'pods', 'services', 'ingresses', 'pvcs', 'events'];
        try {
            const promises = endpoints.map(e => fetch(`/api/${e}`).then(res => res.json()));
            const [overview, nodes, namespaces, pods, services, ingresses, pvcs, events] = await Promise.all(promises);
            
            this.fullDataCache = { overview, nodes, namespaces, pods, services, ingresses, pvcs, events };
            
            this.populateNamespaceSelector();
            this.renderAllSections(); // Renderiza tudo com os novos dados
            this.updateLastUpdated(true);
        } catch (error) {
            console.error("Erro ao buscar dados iniciais:", error);
            this.updateLastUpdated(false);
        }
    }
    
    // Busca apenas métricas, que são mais voláteis
    async fetchMetrics() {
        try {
            const [nodes, pods] = await Promise.all([
                fetch('/api/nodes').then(res => res.json()),
                fetch('/api/pods').then(res => res.json())
            ]);

            // Atualiza o cache de nós e pods com as novas métricas
            this.fullDataCache.nodes = nodes;
            this.fullDataCache.pods = pods;
            
            // Re-renderiza as seções afetadas se estiverem visíveis
            if (this.activeSection === 'nodes') this.renderNodeList();
            if (this.activeSection === 'pods') this.renderPodTable();
        } catch (error) {
            console.error("Erro ao buscar métricas:", error);
        }
    }
    
    // --- Lógica de WebSocket ---

    handleWebSocketMessage(message) {
        const { type, payload } = message;
        const resource = payload.object;
        const eventType = payload.type; // ADDED, MODIFIED, DELETED

        this.flashUpdateIndicator();

        const resourceTypeMap = {
            pods: 'pods', services: 'services', ingresses: 'ingresses',
            pvcs: 'pvcs', events: 'events', nodes: 'nodes'
        };
        const cacheKey = resourceTypeMap[type];
        if (!cacheKey) return;
        
        const idKey = resource.metadata.uid;
        this.updateCache(cacheKey, resource, eventType, idKey);
        
        // Se a seção do recurso atualizado estiver ativa, re-renderiza
        if (this.activeSection === cacheKey) {
            this.renderActiveSection();
        }
    }
    
    updateCache(cacheKey, resource, eventType, idKey) {
        const cache = this.fullDataCache[cacheKey];
        const existingIndex = cache.findIndex(item => item.uid === idKey);

        if (eventType === 'DELETED') {
            if (existingIndex > -1) cache.splice(existingIndex, 1);
            return;
        }

        const processedItem = this.processRawResource(resource, cacheKey);
        if (existingIndex > -1) {
            // Modifica: Mantém os dados antigos (como métricas) e atualiza com os novos do WebSocket
            cache[existingIndex] = { ...cache[existingIndex], ...processedItem };
        } else {
            cache.unshift(processedItem); // Adiciona
        }
    }

    processRawResource(resource, type) {
        if (type === 'pods') {
            const status = resource.status.containerStatuses?.find(cs => cs.state.waiting)?.state.waiting.reason || resource.status.phase;
            const restarts = resource.status.containerStatuses?.reduce((sum, cs) => sum + cs.restartCount, 0) || 0;
            return {
                uid: resource.metadata.uid,
                name: resource.metadata.name,
                namespace: resource.metadata.namespace,
                nodeName: resource.spec.nodeName,
                status: status,
                restarts: restarts,
            };
        }
        if (type === 'events') return this.processEventInfo(resource);
        return resource;
    }
    
    // --- Funções de Renderização (usando cache) ---
    
    renderActiveSection() {
        const renderMap = {
            dashboard: this.renderOverview,
            nodes: this.renderNodeList,
            pods: this.renderPodTable,
            services: this.renderServicesView,
            ingresses: this.renderIngressesView,
            storage: this.renderStorageView,
            events: this.renderEventFeed,
        };
        const renderFunction = renderMap[this.activeSection];
        if (renderFunction) {
            renderFunction.call(this);
        }
    }

    renderAllSections() {
        Object.values(this).filter(v => typeof v === 'function' && v.name.startsWith('render')).forEach(fn => fn.call(this));
    }

    // Retorna uma fatia filtrada do cache
    getFilteredData(cacheKey) {
        if (!this.selectedNamespace) {
            return this.fullDataCache[cacheKey] || [];
        }
        return (this.fullDataCache[cacheKey] || []).filter(item => item.namespace === this.selectedNamespace);
    }
    
    renderOverview() {
        const data = this.fullDataCache.overview;
        if (!data || !Object.keys(data).length) return;
        document.getElementById('running-status').innerHTML = data.isRunningInCluster 
            ? `<span style="color: var(--green-500);">In-Cluster</span>` 
            : `<span style="color: var(--yellow-500);">Local</span>`;
        document.getElementById('nodes-count').innerText = data.nodeCount || 0;
        document.getElementById('deployments-count').innerText = data.deploymentCount || 0;
        document.getElementById('namespaces-count').innerText = data.namespaceCount || 0;
        this.renderCapacityView(data.capacity);
    }

    renderCapacityView(capacity) {
        if (!capacity) return;
        const toGiB = (bytes) => (bytes / (1024 * 1024 * 1024)).toFixed(2);
        const toCores = (milli) => (milli / 1000).toFixed(2);
        
        document.getElementById('cpu-progress-bar').style.width = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        document.getElementById('cpu-usage-text').innerText = `${toCores(capacity.usedCpu)} / ${toCores(capacity.totalCpu)} Cores`;
        document.getElementById('cpu-usage-percentage').innerText = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        
        document.getElementById('memory-progress-bar').style.width = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
        document.getElementById('memory-usage-text').innerText = `${toGiB(capacity.usedMemory)} / ${toGiB(capacity.totalMemory)} GiB`;
        document.getElementById('memory-usage-percentage').innerText = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
    }

    renderNodeList() {
        const nodes = this.fullDataCache.nodes || [];
        const container = document.getElementById('nodes-list');
        container.innerHTML = nodes.map(node => `
            <div class="card node-card">
                <div class="node-header"><h4>${node.name}</h4>${node.role === 'Control-Plane' ? '<span class="node-role">MASTER</span>' : ''}</div>
                <div>
                    <div class="node-metric-label"><span>CPU</span><span style="font-family: monospace;">${node.usedCpu} / ${node.totalCpu}</span></div>
                    <div class="progress-bar-bg"><div class="progress-bar bg-blue" style="width: ${node.cpuUsagePercentage.toFixed(2)}%"></div></div>
                </div>
                <div>
                    <div class="node-metric-label"><span>Memória</span><span style="font-family: monospace;">${node.usedMemory} / ${node.totalMemory}</span></div>
                    <div class="progress-bar-bg"><div class="progress-bar bg-green" style="width: ${node.memoryUsagePercentage.toFixed(2)}%"></div></div>
                </div>
                <div class="node-pods-count"><span>Pods</span><span class="count">${node.podCount}</span></div>
            </div>
        `).join('') || '<p>Nenhum nó encontrado.</p>';
    }

    renderPodTable() {
        const pods = this.getFilteredData('pods');
        const tableBody = document.getElementById('pods-table-body');
        
        this.renderSortableTableHeader('pods-table-header', [
            { name: 'Pod / Namespace', key: 'name' }, { name: 'Nó', key: 'nodeName' }, { name: 'Status', key: 'status' },
            { name: 'Restarts', key: 'restarts' }, { name: 'CPU', key: 'usedCpuMilli' }, { name: 'Memória', key: 'usedMemoryBytes' },
        ], () => this.renderPodTable());

        this.sortData(pods);
        tableBody.innerHTML = pods.map(pod => `
            <tr>
                <td><div><b>${pod.name}</b></div><div class="text-gray">${pod.namespace}</div></td>
                <td class="monospace">${pod.nodeName || 'N/A'}</td>
                <td><span class="status-badge ${this.getPodStatusClass(pod.status)}">${pod.status || 'Unknown'}</span></td>
                <td class="monospace text-center">${pod.restarts}</td>
                <td class="monospace">${pod.usedCpu || '-'}</td>
                <td class="monospace">${pod.usedMemory || '-'}</td>
            </tr>
        `).join('') || `<tr><td colspan="6" class="text-center-padded">Nenhum pod encontrado.</td></tr>`;
    }
    
    renderServicesView() {
        const services = this.getFilteredData('services');
        const tableBody = document.getElementById('services-table-body');
        tableBody.innerHTML = services.map(s => `
            <tr>
                <td>${s.namespace}</td><td><b>${s.name}</b></td><td class="monospace">${s.type}</td>
                <td class="monospace">${s.clusterIp || 'N/A'}</td><td class="monospace">${s.externalIp || 'N/A'}</td><td class="monospace">${s.ports}</td>
            </tr>
        `).join('') || `<tr><td colspan="6" class="text-center-padded">Nenhum serviço encontrado.</td></tr>`;
    }

    renderIngressesView() {
        const ingresses = this.getFilteredData('ingresses');
        const tableBody = document.getElementById('ingresses-table-body');
        tableBody.innerHTML = ingresses.map(i => `
            <tr>
                <td>${i.namespace}</td><td><b>${i.name}</b></td>
                <td class="monospace"><a href="http://${i.hosts.split(',')[0]}" target="_blank" class="link-blue">${i.hosts}</a></td>
                <td class="monospace">${i.service}</td>
            </tr>
        `).join('') || `<tr><td colspan="4" class="text-center-padded">Nenhum ingress encontrado.</td></tr>`;
    }

    renderStorageView() {
        const pvcs = this.getFilteredData('pvcs');
        const tableBody = document.getElementById('pvcs-table-body');
        tableBody.innerHTML = pvcs.map(pvc => `
            <tr>
                <td>${pvc.namespace}</td><td><b>${pvc.name}</b></td>
                <td><span class="status-badge ${pvc.status === 'Bound' ? 'status-bound' : 'status-pending'}">${pvc.status}</span></td>
                <td class="monospace">${pvc.capacity}</td>
            </tr>
        `).join('') || `<tr><td colspan="4" class="text-center-padded">Nenhum PVC encontrado.</td></tr>`;
    }

    renderEventFeed() {
        const events = this.getFilteredData('events');
        const eventsList = document.getElementById('events-list');
        eventsList.innerHTML = events.slice(0, 50).map(event => this.createEventCardHTML(event)).join('') || '<p>Nenhum evento recente.</p>';
    }

    // --- Funções Utilitárias ---

    populateNamespaceSelector() {
        const selector = document.getElementById('namespace-selector');
        selector.innerHTML = '<option value="">Todos os Namespaces</option>'; // Reseta
        (this.fullDataCache.namespaces || []).forEach(ns => {
            const option = document.createElement('option');
            option.value = ns.name;
            option.textContent = ns.name;
            selector.appendChild(option);
        });
    }

    renderSortableTableHeader(headerId, headers, callback) {
        const tableHeader = document.getElementById(headerId);
        tableHeader.innerHTML = headers.map(h => 
            `<th data-key="${h.key}" class="sortable">${h.name} ${this.currentSort.key === h.key ? (this.currentSort.order === 'asc' ? '▲' : '▼') : ''}</th>`
        ).join('');
        
        tableHeader.querySelectorAll('.sortable').forEach(th => {
            th.addEventListener('click', () => {
                const key = th.dataset.key;
                this.currentSort.key === key
                    ? (this.currentSort.order = this.currentSort.order === 'asc' ? 'desc' : 'asc')
                    : (this.currentSort = { key, order: 'asc' });
                callback();
            });
        });
    }

    sortData(data) {
        data.sort((a, b) => {
            let aVal = a[this.currentSort.key], bVal = b[this.currentSort.key];
            if (typeof aVal === 'string') return this.currentSort.order === 'asc' ? aVal.localeCompare(bVal) : bVal.localeCompare(aVal);
            return this.currentSort.order === 'asc' ? (aVal || 0) - (bVal || 0) : (bVal || 0) - (aVal || 0);
        });
    }

    updateConnectionStatus(isConnected) {
        const indicator = document.getElementById('update-indicator');
        indicator.style.backgroundColor = isConnected ? 'var(--green-500)' : 'var(--red-500)';
        console.log(isConnected ? 'Conectado ao servidor WebSocket.' : 'Desconectado do WebSocket.');
    }
    
    updateLastUpdated(isSuccess) {
        document.getElementById('last-updated').innerText = isSuccess
            ? `Atualizado: ${new Date().toLocaleTimeString()}`
            : "Erro ao carregar dados.";
    }

    flashUpdateIndicator() {
        const indicator = document.getElementById('update-indicator');
        indicator.style.backgroundColor = 'var(--blue-500)';
        this.updateLastUpdated(true);
        setTimeout(() => { indicator.style.backgroundColor = 'var(--green-500)'; }, 500);
    }
    
    getPodStatusClass(status) {
        const s = (status || '').toLowerCase();
        if (s.includes('running') || s.includes('succeeded')) return 'status-running';
        if (s.includes('pending') || s.includes('creating')) return 'status-pending';
        if (s.includes('failed') || s.includes('error') || s.includes('crash')) return 'status-failed';
        return 'status-unknown';
    }

    createEventCardHTML(event) {
        const eventTypeBorders = { 'Normal': 'var(--blue-500)', 'Warning': 'var(--yellow-500)' };
        return `
            <div class="card event-card" style="border-left-color: ${eventTypeBorders[event.type] || 'var(--gray-500)'}">
                <div class="event-header"><b>${event.reason}</b><span>${new Date(event.timestamp).toLocaleString()}</span></div>
                <p class="event-message"><b>${event.object}:</b> ${event.message}</p>
            </div>`;
    }
    
    processEventInfo(event) {
        return {
            timestamp: event.lastTimestamp || event.eventTime,
            type: event.type,
            reason: event.reason,
            object: `${event.involvedObject.kind}/${event.involvedObject.name}`,
            message: event.message,
            namespace: event.involvedObject.namespace
        };
    }
}
