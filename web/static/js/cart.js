(() => {
    "use strict";

    const STORAGE_KEY = "gorras_cart_v1";
    const bodyPage = document.body.dataset.page || "";
    const copFormatter = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 0 });

    function formatCOP(value) {
        return "$" + copFormatter.format(Number(value) || 0) + " COP";
    }

    function readCart() {
        try {
            const raw = JSON.parse(localStorage.getItem(STORAGE_KEY) || "[]");
            if (!Array.isArray(raw)) {
                return [];
            }

            const merged = new Map();
            raw.forEach((item) => {
                const id = Number.parseInt(item && item.id, 10);
                const quantity = Number.parseInt(item && item.quantity, 10);
                if (Number.isInteger(id) && id > 0 && Number.isInteger(quantity) && quantity > 0) {
                    merged.set(id, (merged.get(id) || 0) + quantity);
                }
            });

            return Array.from(merged, ([id, quantity]) => ({ id, quantity }));
        } catch {
            return [];
        }
    }

    function writeCart(items) {
        try {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
        } catch {
            // The UI still updates even if storage is unavailable.
        }
        updateCartCount(items);
    }

    function updateCartCount(items = readCart()) {
        const count = items.reduce((total, item) => total + item.quantity, 0);
        document.querySelectorAll("[data-cart-count]").forEach((element) => {
            element.textContent = String(count);
        });
    }

    function clampQuantity(value, max) {
        const parsed = Number.parseInt(value, 10);
        const quantity = Number.isInteger(parsed) ? parsed : 1;
        return Math.max(1, Math.min(quantity, max));
    }

    if (bodyPage === "payment-success") {
        try {
            localStorage.removeItem(STORAGE_KEY);
        } catch {
            // Ignore unavailable storage.
        }
        updateCartCount([]);
    } else {
        updateCartCount();
    }

    setupModal();
    setupCartPage();

    function setupModal() {
        const modal = document.getElementById("cap-modal");
        if (!modal) {
            return;
        }

        const cards = Array.from(document.querySelectorAll(".cap-card"));
        const image = document.getElementById("modal-image");
        const name = document.getElementById("modal-name");
        const description = document.getElementById("modal-description");
        const color = document.getElementById("modal-color");
        const price = document.getElementById("modal-price");
        const stock = document.getElementById("modal-stock");
        const quantity = document.getElementById("modal-quantity");
        const addButton = document.getElementById("modal-add");
        const message = document.getElementById("modal-message");
        const closeButton = document.getElementById("modal-close");
        let activeCap = null;
        let lastFocused = null;

        function openModal(card) {
            const capStock = Number.parseInt(card.dataset.stock, 10) || 0;
            activeCap = {
                id: Number.parseInt(card.dataset.id, 10),
                name: card.dataset.name || "",
                price: card.dataset.priceDisplay || "",
                stock: capStock,
            };
            lastFocused = document.activeElement;

            image.src = card.dataset.image || "";
            image.alt = card.dataset.name || "";
            name.textContent = card.dataset.name || "";
            description.textContent = card.dataset.description || "";
            color.textContent = card.dataset.color || "";
            price.textContent = card.dataset.priceDisplay || "";
            stock.textContent = capStock > 0
                ? "Unidades disponibles: " + capStock
                : "Agotada";
            quantity.value = "1";
            quantity.max = String(Math.max(capStock, 1));
            quantity.disabled = capStock === 0;
            addButton.disabled = capStock === 0;
            addButton.textContent = capStock === 0 ? "Agotada" : "Agregar al carrito";
            message.textContent = "";

            modal.hidden = false;
            document.body.style.overflow = "hidden";
            closeButton.focus();
        }

        function closeModal() {
            modal.hidden = true;
            document.body.style.overflow = "";
            activeCap = null;
            if (lastFocused && typeof lastFocused.focus === "function") {
                lastFocused.focus();
            }
        }

        cards.forEach((card) => {
            card.addEventListener("click", () => openModal(card));
            card.addEventListener("keydown", (event) => {
                if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    openModal(card);
                }
            });
        });

        modal.querySelectorAll("[data-modal-close]").forEach((element) => {
            element.addEventListener("click", closeModal);
        });

        document.addEventListener("keydown", (event) => {
            if (event.key === "Escape" && !modal.hidden) {
                closeModal();
            }
        });

        quantity.addEventListener("change", () => {
            if (activeCap) {
                quantity.value = String(clampQuantity(quantity.value, activeCap.stock));
            }
        });

        addButton.addEventListener("click", () => {
            if (!activeCap || activeCap.stock === 0) {
                return;
            }

            const requested = clampQuantity(quantity.value, activeCap.stock);
            const items = readCart();
            const existing = items.find((item) => item.id === activeCap.id);
            const currentQuantity = existing ? existing.quantity : 0;
            const nextQuantity = Math.min(currentQuantity + requested, activeCap.stock);

            if (nextQuantity === currentQuantity) {
                message.textContent = "Ya tienes el máximo disponible en el carrito.";
                return;
            }

            if (existing) {
                existing.quantity = nextQuantity;
            } else {
                items.push({ id: activeCap.id, quantity: nextQuantity });
            }
            writeCart(items);
            message.textContent = "Agregado al carrito.";
        });
    }

    function setupCartPage() {
        const content = document.getElementById("cart-content");
        if (!content) {
            return;
        }

        const loading = document.getElementById("cart-loading");
        const empty = document.getElementById("cart-empty");
        const itemsBody = document.getElementById("cart-items");
        const total = document.getElementById("cart-total");
        const checkoutButton = document.getElementById("checkout-button");
        const message = document.getElementById("cart-message");

        function showError(text) {
            message.textContent = text;
            message.hidden = !text;
        }

        function updateQuantity(id, quantity) {
            const items = readCart();
            const item = items.find((entry) => entry.id === id);
            if (!item) {
                return;
            }
            item.quantity = quantity;
            writeCart(items);
            loadCart();
        }

        function removeItem(id) {
            const items = readCart().filter((item) => item.id !== id);
            writeCart(items);
            loadCart();
        }

        function render(items) {
            itemsBody.replaceChildren();
            items.forEach((item) => {
                const row = document.createElement("tr");

                const productCell = document.createElement("td");
                const product = document.createElement("div");
                product.className = "cart-product";

                const image = document.createElement("img");
                image.src = item.image_path;
                image.alt = item.name;
                product.appendChild(image);

                const names = document.createElement("div");
                const itemName = document.createElement("div");
                itemName.className = "cart-product-name";
                itemName.textContent = item.name;
                const itemColor = document.createElement("div");
                itemColor.className = "cart-product-color";
                itemColor.textContent = item.color;
                names.append(itemName, itemColor);
                product.appendChild(names);
                productCell.appendChild(product);

                const priceCell = document.createElement("td");
                priceCell.textContent = formatCOP(item.price);

                const quantityCell = document.createElement("td");
                const quantityInput = document.createElement("input");
                quantityInput.type = "number";
                quantityInput.className = "cart-quantity";
                quantityInput.min = "1";
                quantityInput.max = String(item.stock);
                quantityInput.value = String(item.quantity);
                quantityInput.setAttribute("aria-label", "Cantidad de " + item.name);
                quantityInput.addEventListener("change", () => {
                    const quantity = clampQuantity(quantityInput.value, item.stock);
                    quantityInput.value = String(quantity);
                    if (quantity !== item.quantity) {
                        updateQuantity(item.id, quantity);
                    }
                });
                quantityCell.appendChild(quantityInput);

                const subtotalCell = document.createElement("td");
                subtotalCell.textContent = formatCOP(item.line_total);

                const actionCell = document.createElement("td");
                const removeButton = document.createElement("button");
                removeButton.type = "button";
                removeButton.className = "button-link";
                removeButton.textContent = "Quitar";
                removeButton.addEventListener("click", () => removeItem(item.id));
                actionCell.appendChild(removeButton);

                row.append(productCell, priceCell, quantityCell, subtotalCell, actionCell);
                itemsBody.appendChild(row);
            });

            total.textContent = formatCOP(items.reduce((sum, item) => sum + item.line_total, 0));
        }

        async function loadCart() {
            const items = readCart();
            loading.hidden = false;
            showError("");
            content.hidden = true;
            empty.hidden = true;

            if (items.length === 0) {
                loading.hidden = true;
                empty.hidden = false;
                return;
            }

            try {
                const response = await fetch("/api/cart", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ items }),
                });
                const data = await response.json().catch(() => ({}));
                if (!response.ok) {
                    throw new Error(data.error || "No se pudo cargar el carrito.");
                }

                loading.hidden = true;
                render(Array.isArray(data.items) ? data.items : []);
                content.hidden = false;
            } catch (error) {
                loading.hidden = true;
                showError(error.message || "No se pudo cargar el carrito.");
            }
        }

        checkoutButton.addEventListener("click", async () => {
            const items = readCart();
            if (items.length === 0) {
                await loadCart();
                return;
            }

            const originalLabel = checkoutButton.textContent;
            checkoutButton.disabled = true;
            checkoutButton.textContent = "Redirigiendo a Mercado Pago…";
            showError("");

            try {
                const response = await fetch("/checkout", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ items }),
                });
                const data = await response.json().catch(() => ({}));
                if (!response.ok) {
                    throw new Error(data.error || "No se pudo iniciar el pago.");
                }
                if (!data.init_point) {
                    throw new Error("Mercado Pago no devolvió una URL de pago.");
                }
                window.location.assign(data.init_point);
            } catch (error) {
                showError(error.message || "No se pudo iniciar el pago.");
                checkoutButton.disabled = false;
                checkoutButton.textContent = originalLabel;
            }
        });

        loadCart();
    }
})();
