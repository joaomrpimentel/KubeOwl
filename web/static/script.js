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
        this.isMetricsFetching = false; // Flag para evitar sobreposição de fetch
    }

    init() {
        this.setupTheme();
        this.setupNavigation();
        this.setupNamespaceSelector();
        this.loadData(); // Orquestra o carregamento de dados em estágios
        this.setupWebSocket();
        
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

        const initialIsDataSection = ['pods', 'services', 'ingresses', 'storage', 'events'].includes(this.activeSection);
        namespaceSelectorContainer.style.display = initialIsDataSection ? 'flex' : 'none';

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

                this.renderActiveSection();
            });
        });
    }

    setupNamespaceSelector() {
        const selector = document.getElementById('namespace-selector');
        selector.addEventListener('change', (e) => {
            this.selectedNamespace = e.target.value;
            this.renderActiveSection();
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

    async loadData() {
        try {
            await this.fetchInitialData();
            this.fetchSecondaryData();
        } catch (error) {
            console.error("Falha no carregamento inicial, o carregamento secundário foi cancelado.", error);
        }
    }
    
    async fetchInitialData() {
        const endpoints = ['overview', 'nodes', 'namespaces'];
        try {
            this.updateLastUpdated(null);
            const [overview, nodes, namespaces] = await Promise.all(
                endpoints.map(e => fetch(`/api/${e}`).then(res => res.json()))
            );
            
            this.fullDataCache.overview = overview;
            this.fullDataCache.nodes = nodes;
            this.fullDataCache.namespaces = namespaces;
            
            this.populateNamespaceSelector();
            this.renderActiveSection();
            
        } catch (error) {
            this.updateLastUpdated(false);
            throw error;
        }
    }

    async fetchSecondaryData() {
        const endpoints = ['pods', 'services', 'ingresses', 'pvcs', 'events'];
        try {
            const [pods, services, ingresses, pvcs, events] = await Promise.all(
                endpoints.map(e => fetch(`/api/${e}`).then(res => res.json()))
            );
            
            this.fullDataCache.pods = pods;
            this.fullDataCache.services = services;
            this.fullDataCache.ingresses = ingresses;
            this.fullDataCache.pvcs = pvcs;
            this.fullDataCache.events = events;

            this.updateLastUpdated(true);
            
            this.renderActiveSection();

        } catch (error) {
            console.error("Erro ao buscar dados secundários:", error);
            this.updateLastUpdated(false);
        }
    }
    
    async fetchMetrics() {
        if (this.isMetricsFetching) return;
        this.isMetricsFetching = true;

        try {
            const [nodes, pods] = await Promise.all([
                fetch('/api/nodes').then(res => res.json()),
                fetch('/api/pods').then(res => res.json())
            ]);

            this.fullDataCache.nodes = nodes;
            this.fullDataCache.pods = pods;
            
            if (this.activeSection === 'nodes') this.renderNodeList();
            if (this.activeSection === 'pods') this.renderPodTable();
        } catch (error) {
            console.error("Erro ao buscar métricas:", error);
        } finally {
            this.isMetricsFetching = false;
        }
    }
    
    // --- Lógica de WebSocket ---

    handleWebSocketMessage(message) {
        const { type, payload } = message;
        const resource = payload.object;
        const eventType = payload.type;

        this.flashUpdateIndicator();

        const resourceTypeMap = {
            pods: 'pods', services: 'services', ingresses: 'ingresses',
            pvcs: 'pvcs', events: 'events', nodes: 'nodes'
        };
        const cacheKey = resourceTypeMap[type];
        if (!cacheKey) return;
        
        const idKey = resource.metadata.uid;
        this.updateCache(cacheKey, resource, eventType, idKey);
        
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
            cache[existingIndex] = { ...cache[existingIndex], ...processedItem };
        } else {
            cache.unshift(processedItem);
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
    
    // --- Funções de Renderização Segura ---

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
        document.getElementById('nodes-count').textContent = data.nodeCount || 0;
        document.getElementById('deployments-count').textContent = data.deploymentCount || 0;
        document.getElementById('namespaces-count').textContent = data.namespaceCount || 0;
        this.renderCapacityView(data.capacity);
    }

    renderCapacityView(capacity) {
        if (!capacity) return;
        const toGiB = (bytes) => (bytes / (1024 * 1024 * 1024)).toFixed(2);
        const toCores = (milli) => (milli / 1000).toFixed(2);
        
        document.getElementById('cpu-progress-bar').style.width = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        document.getElementById('cpu-usage-text').textContent = `${toCores(capacity.usedCpu)} / ${toCores(capacity.totalCpu)} Cores`;
        document.getElementById('cpu-usage-percentage').textContent = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        
        document.getElementById('memory-progress-bar').style.width = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
        document.getElementById('memory-usage-text').textContent = `${toGiB(capacity.usedMemory)} / ${toGiB(capacity.totalMemory)} GiB`;
        document.getElementById('memory-usage-percentage').textContent = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
    }

    renderNodeList() {
        const nodes = this.fullDataCache.nodes || [];
        const container = document.getElementById('nodes-list');
        container.innerHTML = ''; // Limpa antes de adicionar
        if (nodes.length === 0) {
            container.innerHTML = '<p>Nenhum nó encontrado.</p>';
            return;
        }
        nodes.forEach(node => {
            const card = document.createElement('div');
            card.className = 'card node-card';
            card.innerHTML = `
                <div class="node-header">
                    <h4></h4>
                    ${node.role === 'Control-Plane' ? '<span class="node-role">MASTER</span>' : ''}
                </div>
                <div>
                    <div class="node-metric-label"><span>CPU</span><span class="monospace"></span></div>
                    <div class="progress-bar-bg"><div class="progress-bar bg-blue" style="width: ${node.cpuUsagePercentage.toFixed(2)}%"></div></div>
                </div>
                <div>
                    <div class="node-metric-label"><span>Memória</span><span class="monospace"></span></div>
                    <div class="progress-bar-bg"><div class="progress-bar bg-green" style="width: ${node.memoryUsagePercentage.toFixed(2)}%"></div></div>
                </div>
                <div class="node-pods-count"><span>Pods</span><span class="count"></span></div>`;
            card.querySelector('h4').textContent = node.name;
            const metrics = card.querySelectorAll('.monospace');
            metrics[0].textContent = `${node.usedCpu} / ${node.totalCpu}`;
            metrics[1].textContent = `${node.usedMemory} / ${node.totalMemory}`;
            card.querySelector('.count').textContent = node.podCount;
            container.appendChild(card);
        });
    }

    renderPodTable() {
        const pods = this.getFilteredData('pods');
        const tableBody = document.getElementById('pods-table-body');
        
        this.renderSortableTableHeader('pods-table-header', [
            { name: 'Pod / Namespace', key: 'name' }, { name: 'Nó', key: 'nodeName' }, { name: 'Status', key: 'status' },
            { name: 'Restarts', key: 'restarts' }, { name: 'CPU', key: 'usedCpuMilli' }, { name: 'Memória', key: 'usedMemoryBytes' },
        ], () => this.renderPodTable());

        this.sortData(pods);
        tableBody.innerHTML = ''; // Limpa
        if (pods.length === 0) {
            tableBody.innerHTML = `<tr><td colspan="6" class="text-center-padded">Nenhum pod encontrado.</td></tr>`;
            return;
        }
        pods.forEach(pod => {
            const row = tableBody.insertRow();
            row.insertCell().innerHTML = `<div><b>${this.escapeHTML(pod.name)}</b></div><div class="text-gray">${this.escapeHTML(pod.namespace)}</div>`;
            row.insertCell().textContent = pod.nodeName || 'N/A';
            row.cells[1].className = 'monospace';
            row.insertCell().innerHTML = `<span class="status-badge ${this.getPodStatusClass(pod.status)}">${this.escapeHTML(pod.status) || 'Unknown'}</span>`;
            row.insertCell().textContent = pod.restarts;
            row.cells[3].className = 'monospace text-center';
            row.insertCell().textContent = pod.usedCpu || '-';
            row.cells[4].className = 'monospace';
            row.insertCell().textContent = pod.usedMemory || '-';
            row.cells[5].className = 'monospace';
        });
    }
    
    renderServicesView() {
        const services = this.getFilteredData('services');
        const tableBody = document.getElementById('services-table-body');
        tableBody.innerHTML = ''; // Limpa
        if (services.length === 0) {
            tableBody.innerHTML = `<tr><td colspan="6" class="text-center-padded">Nenhum serviço encontrado.</td></tr>`;
            return;
        }
        services.forEach(service => {
            const row = tableBody.insertRow();
            row.insertCell().textContent = service.namespace;
            row.insertCell().innerHTML = `<b>${this.escapeHTML(service.name)}</b>`;
            row.insertCell().textContent = service.type;
            row.cells[2].className = 'monospace';
            row.insertCell().textContent = service.clusterIp || 'N/A';
            row.cells[3].className = 'monospace';
            row.insertCell().textContent = service.externalIp || 'N/A';
            row.cells[4].className = 'monospace';
            row.insertCell().textContent = service.ports;
            row.cells[5].className = 'monospace';
        });
    }

    renderIngressesView() {
        const ingresses = this.getFilteredData('ingresses');
        const tableBody = document.getElementById('ingresses-table-body');
        tableBody.innerHTML = ''; // Limpa
        if (ingresses.length === 0) {
            tableBody.innerHTML = `<tr><td colspan="4" class="text-center-padded">Nenhum ingress encontrado.</td></tr>`;
            return;
        }
        ingresses.forEach(ingress => {
            const row = tableBody.insertRow();
            row.insertCell().textContent = ingress.namespace;
            row.insertCell().innerHTML = `<b>${this.escapeHTML(ingress.name)}</b>`;
            const hostCell = row.insertCell();
            const hostLink = document.createElement('a');
            hostLink.href = `http://${ingress.hosts.split(',')[0]}`;
            hostLink.target = '_blank';
            hostLink.className = 'link-blue monospace';
            hostLink.textContent = ingress.hosts;
            hostCell.appendChild(hostLink);
            row.insertCell().textContent = ingress.service;
            row.cells[3].className = 'monospace';
        });
    }

    renderStorageView() {
        const pvcs = this.getFilteredData('pvcs');
        const tableBody = document.getElementById('pvcs-table-body');
        tableBody.innerHTML = ''; // Limpa
        if (pvcs.length === 0) {
            tableBody.innerHTML = `<tr><td colspan="4" class="text-center-padded">Nenhum PVC encontrado.</td></tr>`;
            return;
        }
        pvcs.forEach(pvc => {
            const row = tableBody.insertRow();
            row.insertCell().textContent = pvc.namespace;
            row.insertCell().innerHTML = `<b>${this.escapeHTML(pvc.name)}</b>`;
            row.insertCell().innerHTML = `<span class="status-badge ${pvc.status === 'Bound' ? 'status-bound' : 'status-pending'}">${this.escapeHTML(pvc.status)}</span>`;
            row.insertCell().textContent = pvc.capacity;
            row.cells[3].className = 'monospace';
        });
    }
    
    renderEventFeed() {
        const events = this.getFilteredData('events');
        const eventsList = document.getElementById('events-list');
        eventsList.innerHTML = ''; // Limpa
        if (events.length === 0) {
            eventsList.innerHTML = '<p>Nenhum evento recente.</p>';
            return;
        }
        events.slice(0, 50).forEach(event => {
            const card = this.createEventCard(event);
            eventsList.appendChild(card);
        });
    }

    createEventCard(event) {
        const card = document.createElement('div');
        card.className = 'card event-card';
        const eventTypeBorders = { 'Normal': 'var(--blue-500)', 'Warning': 'var(--yellow-500)' };
        card.style.borderLeftColor = eventTypeBorders[event.type] || 'var(--gray-500)';

        const header = document.createElement('div');
        header.className = 'event-header';
        const reason = document.createElement('b');
        reason.textContent = event.reason;
        const timestamp = document.createElement('span');
        timestamp.textContent = new Date(event.timestamp).toLocaleString();
        header.appendChild(reason);
        header.appendChild(timestamp);

        const messageP = document.createElement('p');
        messageP.className = 'event-message';
        const objectB = document.createElement('b');
        objectB.textContent = `${event.object}: `;
        messageP.appendChild(objectB);
        messageP.append(document.createTextNode(event.message));

        card.appendChild(header);
        card.appendChild(messageP);
        return card;
    }

    // --- Funções Utilitárias ---
    
    escapeHTML(str) {
        if (!str) return '';
        const p = document.createElement('p');
        p.appendChild(document.createTextNode(str));
        return p.innerHTML;
    }

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
        const span = document.getElementById('last-updated');
        if (isSuccess === null) {
            span.innerText = "Carregando...";
        } else {
            span.innerText = isSuccess
                ? `Atualizado: ${new Date().toLocaleTimeString()}`
                : "Erro ao carregar dados.";
        }
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
