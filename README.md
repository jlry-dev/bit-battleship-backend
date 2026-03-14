# Bit Battleships - Backend

This is the high-performance, real-time multiplayer backend for **Bit Battleships**. It is written purely in Go utilizing the standard library, Gorilla WebSockets, and PostgreSQL for persistent match history.

It features a custom-built, thread-safe memory architecture capable of hosting thousands of concurrent websocket connections across hundreds of active game rooms with isolated goroutines.

## 🚀 Getting Started Locally

1. **Install Dependencies:**
   Ensure you have Go 1.22+ installed.
   ```bash
   go mod download
   ```

2. **Configure Environment:**
   Copy `.env.example` to `.env` and set up your local PostgreSQL connection string.
   ```bash
   cp .env.example .env
   ```

3. **Run the Server:**
   ```bash
   go run main.go
   ```
   *The server will start on port 8080. It will automatically run database migrations on startup.*

## 🌐 Deploying to Koyeb

This project is fully optimized for Koyeb's serverless platform. Koyeb natively supports Go applications and WebSocket connections out of the box.

1. **Push to GitHub:** Ensure this repository is pushed to GitHub.
2. **Create Koyeb Service:** In your Koyeb dashboard, click **Create Service** and select your GitHub repository.
3. **Configure the Build:**
   - **Builder:** Select **Dockerfile**. Koyeb will use the provided `Dockerfile` to securely build and run your backend without relying on messy buildpacks.
   - **Docker Context:** Set the context/work directory to `battleship-backend` if your repository has the backend in a subfolder.
4. **Set Environment Variables:**
   Add the following variables in the Koyeb dashboard:
   - `PORT`: `8080` (or whatever port Koyeb assigns, it usually handles this automatically).
   - `DATABASE_URL`: Add your managed PostgreSQL connection string (e.g., from Neon, Supabase, or Koyeb's managed Postgres). Make sure it ends with `?sslmode=require`.
   - `MATCHMAKER_WORKERS`: `4`
5. **Deploy:** Click **Deploy**. Once live, your backend will be accessible via `wss://your-app-name.koyeb.app/ws`.

## 📈 Stress Testing
We have included a custom load tester to verify the backend's concurrency limits.
See [STRESS_TEST_GUIDE.md](./STRESS_TEST_GUIDE.md) for detailed instructions on bombarding the backend with thousands of virtual players.
