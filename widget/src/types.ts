export interface Message {
  id: string; conversation_id: string; sender_id: string; seq: number;
  body: string; client_msg_id?: string; created_at: string;
}
export interface Conversation {
  id: string; peer_id: string; peer_last_read_seq: number; peer_ended: boolean;
}
